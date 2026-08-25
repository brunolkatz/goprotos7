package api

import (
	"context"
	"fmt"
	"github.com/ascarter/requestid"
	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
	"net/http"
	"time"
)

type HttpApiServer struct {
	Router *chi.Mux

	ctx        context.Context
	listenAddr string
	server     *http.Server
}

// NewHTTPServer initializes a (chi) router with all the commonly used options it also adds a
// health check route and auto tracing on the server. It returns the instance of the server to
// be started on blocking or non-blocking mode (Serve or ServeForErrGroup).
func NewHTTPServer(ctx context.Context, port string) (*HttpApiServer, error) {

	if port == "" {
		port = ":8080"
	}

	// Http Server
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(requestid.RequestIDHandler)
	r.Use(middleware.NoCache)
	r.Use(middleware.StripSlashes)
	r.Use(cors.New(
		cors.Options{
			AllowedOrigins: []string{"*"},
			AllowedHeaders: []string{"*"},
			Debug:          false,
			AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions, http.MethodDelete}},
	).Handler)

	// Live probe
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &HttpApiServer{
		Router:     r,
		ctx:        ctx,
		listenAddr: port,
		server:     srv,
	}, nil
}

func (s *HttpApiServer) StopServer() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		log.Errorf("[ERROR] graceful shutdown failed: %v", err)
	}
}

// ListRoutes lists all the registered routes on the server. It makes use of a (chi)
// walk method.
func (s *HttpApiServer) ListRoutes() error {
	if err := chi.Walk(s.Router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		fmt.Printf("[LISTED_ROUTES] [%s] \"%s\"\n", method, route)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// ServeForErrGroup return a function compatible with the format of a ErrGroup, so you
// can run it inside it.
func (s *HttpApiServer) ServeForErrGroup() func() error {
	return func() error {
		err := s.serve()
		if err != nil {
			return err
		}
		return nil
	}
}

func (s *HttpApiServer) serve() error {
	go func() {
		<-s.ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	log.Infof("[INFO] >>>>>>>>>>>>>> Start listening at \"%s\"", s.listenAddr)
	err := s.server.ListenAndServe()
	if err == nil || err == http.ErrServerClosed {
		return nil
	}
	log.Errorf("[ERROR] couldn't start debug http- %v", err)
	return fmt.Errorf("couldn't start debug http- %w", err)
}
