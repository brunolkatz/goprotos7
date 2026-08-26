package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/api"
	"github.com/brunolkatz/goprotos7/dbtool/db/sqlite_db"
	"github.com/brunolkatz/goprotos7/dbtool/handlers/data-block-handlers"
	vars_handler "github.com/brunolkatz/goprotos7/dbtool/handlers/vars-handler"
	ws_handler "github.com/brunolkatz/goprotos7/dbtool/handlers/ws-handler"
	"github.com/brunolkatz/goprotos7/dbtool/internals/browser"
	"github.com/charmbracelet/log"
	"github.com/jessevdk/go-flags"
	"golang.org/x/sync/errgroup"
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

	db, err := sql_lite_db.New(ctx, webAdminConfig.SQLiteFilePath, logger, !webAdminConfig.Flags.SilentDBLogs)
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
		wsServer, err := ws_handler.New(varsHandler, ":3001")
		if err != nil {
			panic(err)
		}
		httpServer, err := api.NewHTTPServer(ctx, ":8080")
		if err != nil {
			panic(err)
		}
		if err := registerWebAdminRoutes(httpServer.Router, varsHandler); err != nil {
			panic(err)
		}

		g.Go(func() error {
			// Ugly hack to make sure the server is initialized before returning
			// This works like the group.Go() function in errgroup package
			if err = httpServer.ServeForErrGroup()(); err != nil {
				logger.Errorf("Error serving http server on port %s - Error: %+v", "8080", err)
				return err
			}
			return nil
		})
		g.Go(wsServer.ServeForErrGroup())

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
					httpServer.StopServer()
					_ = wsServer.Stop(context.Background())
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
