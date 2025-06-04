package api

import (
	"bufio"
	"context"
	"fmt"
	"github.com/ascarter/requestid"
	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type HttpApiServer struct {
	Router   *chi.Mux
	ctx      context.Context
	port     string
	stopChan chan bool
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

	return &HttpApiServer{
		Router:   r,
		ctx:      ctx,
		port:     port,
		stopChan: make(chan bool, 1),
	}, nil
}

func (s *HttpApiServer) StopServer() {
	s.stopChan <- true
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
		err := serve(s.ctx, s.port, s.Router, s.stopChan)
		if err != nil {
			return err
		}
		return nil
	}
}

func serve(ctx context.Context, listenAddr string, router *chi.Mux, stopChan chan bool) error {
	errCh := make(chan error)

	// Run HTTP Server
	go func() {
		errCh <- http.ListenAndServe(listenAddr, router)
	}()

	// Run CheckPort
	go func() {
		path := strings.Split(listenAddr, ":")
		host := "127.0.0.1"
		if len(path) == 1 {
			host = path[0]
		}
		if len(path) > 1 {
			host = strings.Join([]string{host, path[1]}, ":")
		}
		for checkPort(host) {
			time.Sleep(30 * time.Millisecond)
		}
		log.Infof("[INFO] >>>>>>>>>>>>>> Start listening at \"%s\"", listenAddr)
	}()

	// Wait for it to Finish
	select {
	case <-ctx.Done():
		return nil
	case <-stopChan:
		log.Warnf("[WAR] http server stopped")
		return nil
	case err := <-errCh:
		log.Errorf("[ERROR] couldn't start debug http- %v", err)
		return fmt.Errorf("couldn't start debug http- %w", err)
	}
}

func checkPort(addr string) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}

	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			panic(err)
		}
	}(conn)

	_, _, err = bufio.NewReader(conn).ReadLine()
	if err != nil {
		if err == io.EOF {
			return true
		}
	}
	return false
}
