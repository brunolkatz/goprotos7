package assets_files_watcher

import (
	"context"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
)

type AssetsFilesWatcherApiHandler struct {
	baseCtx context.Context
	fs      http.FileSystem
}

func New(baseCtx context.Context) (*AssetsFilesWatcherApiHandler, error) {
	assetsFS, err := dbtool.AssetsFileSystem()
	if err != nil {
		return nil, err
	}
	return &AssetsFilesWatcherApiHandler{
		baseCtx: baseCtx,
		fs:      assetsFS,
	}, nil
}

func (h *AssetsFilesWatcherApiHandler) Register(router *chi.Mux) {
	router.Handle("/assets/*", http.StripPrefix("/assets", middleware.Compress(5, "text/css", "application/javascript")(http.FileServer(h.fs))))
}
