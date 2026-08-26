package assets_files_watcher

import (
	"context"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type AssetsFilesWatcherApiHandler struct {
	baseCtx context.Context
}

func New(baseCtx context.Context) (*AssetsFilesWatcherApiHandler, error) {
	return &AssetsFilesWatcherApiHandler{
		baseCtx: baseCtx,
	}, nil
}

func (h *AssetsFilesWatcherApiHandler) Register(router *chi.Mux) {
	router.Handle("/assets/*", http.StripPrefix("/assets", http.FileServer(AssetFile())))
}
