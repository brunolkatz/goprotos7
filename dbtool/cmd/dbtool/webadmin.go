package main

import (
	"context"
	assets_files_watcher "github.com/brunolkatz/goprotos7/dbtool/api/assets-files-watcher-api"
	create_var_api "github.com/brunolkatz/goprotos7/dbtool/api/create-var-api"
	dashboard_api "github.com/brunolkatz/goprotos7/dbtool/api/dashboard-api"
	vars_handler "github.com/brunolkatz/goprotos7/dbtool/handlers/vars-handler"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func registerWebAdminRoutes(router *chi.Mux, varsHandler *vars_handler.VarsHandler) error {
	assetsFilesWatcherApi, err := assets_files_watcher.New(context.Background())
	if err != nil {
		return err
	}
	dashboardApi, err := dashboard_api.New(varsHandler)
	if err != nil {
		return err
	}
	createVarApi, err := create_var_api.New(varsHandler)
	if err != nil {
		return err
	}

	assetsFilesWatcherApi.Register(router)
	router.Route("/", func(r chi.Router) {
		r.Use(middleware.SetHeader("Content-Type", "text/html; charset=utf-8"))
		dashboardApi.Register(r)
		createVarApi.Register(r)
	})
	return nil
}
