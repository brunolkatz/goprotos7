package main

import (
	"context"
	"errors"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/api"
	"github.com/brunolkatz/goprotos7/dbtool/api/assets-files-watcher-api"
	create_var_api "github.com/brunolkatz/goprotos7/dbtool/api/create-var-api"
	"github.com/brunolkatz/goprotos7/dbtool/api/dashboard-api"
	"github.com/brunolkatz/goprotos7/dbtool/db/sqlite_db"
	"github.com/brunolkatz/goprotos7/dbtool/handlers/data-block-handlers"
	vars_handler "github.com/brunolkatz/goprotos7/dbtool/handlers/vars-handler"
	"github.com/brunolkatz/goprotos7/dbtool/internals/browser"
	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jessevdk/go-flags"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	webAdminConfig := dbtool.Config{}
	parser := flags.NewParser(&webAdminConfig, flags.Default)
	if _, err := parser.Parse(); err != nil {
		panic(err)
	}

	logOptions := log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.DateTime,
		Level:           log.DebugLevel,
	}
	logger := log.NewWithOptions(os.Stderr, logOptions)

	// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	// ┃                                              Initialize Database                                              ┃
	// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

	db, err := sql_lite_db.New(ctx, webAdminConfig.SQLiteFilePath, logger)
	if err != nil {
		panic(err)
	}

	dbHandler, err := data_block_handlers.New(ctx, webAdminConfig.DBBinPaths, db, logger)
	if err != nil {
		panic(err)
	}
	logger.Infof("Creating database blocks with bin paths: %v", webAdminConfig.DBBinPaths)
	err = dbHandler.CreateDatabaseBlocks()
	if err != nil {
		panic(err)
	}
	logger.Infof("Database blocks created/updated successfully")

	// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	// ┃                                          Initialize databases blocks                                          ┃
	// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

	// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	// ┃                                           Initialize HTTP handlers                                            ┃
	// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

	if webAdminConfig.Flags.EnableWebAdmin {

		varsHandler, err := vars_handler.New(db, dbHandler, logger)
		if err != nil {
			logger.Errorf("Error creating vars handler: %v", err)
			panic(err)
		}

		logger.Infof("Webadmin enabled, starting web admin...")
		httpServer, err := api.NewHTTPServer(ctx, ":8080")
		if err != nil {
			panic(err)
		}

		assetsFilesWatcherApi, err := assets_files_watcher.New(ctx)
		if err != nil {
			panic(err)
		}

		dasboardApi, err := dashboard_api.New(varsHandler)
		if err != nil {
			panic(err)
		}

		createVarApi, err := create_var_api.New(varsHandler)
		if err != nil {
			panic(err)
		}

		assetsFilesWatcherApi.Register(httpServer.Router)

		// Register the HTML Pages
		// Add all HTML pages here
		httpServer.Router.Route("/", func(r chi.Router) {
			r.Use(middleware.SetHeader("Content-Type", "text/html; charset=utf-8"))
			dasboardApi.Register(r)  // Register the dashboard page
			createVarApi.Register(r) // Register the create variable page
		})

		g.Go(func() error {
			// Ugly hack to make sure the server is initialized before returning
			// This works like the group.Go() function in errgroup package
			if err = httpServer.ServeForErrGroup()(); err != nil {
				logger.Errorf("Error serving http server on port %s - Error: %+v", "8080", err)
				return err
			}
			return nil
		})

		var errStop = errors.New("stop")
		g.Go(func() error {
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT)
			for {
				select {
				case <-ctx.Done():
					return errStop
				case _ = <-sigs:
					cancel()
				}
			}
		})

		_ = browser.Open("http://localhost:8080") // open the browser automatically

		logger.Infof("Webadmin initialized...")
		if err = g.Wait(); err != nil || errors.Is(err, errStop) {
			logger.Errorf("Error running errgroup: %v", err)
			return
		}
		log.Info("WebAdmin server stopped")
	} else {
		log.Warnf("WebAdmin server disabled, exiting...")
	}
}
