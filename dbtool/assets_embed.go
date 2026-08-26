package dbtool

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/css/index.css
//go:embed assets/js/htmx.min.js
//go:embed assets/js/dist/*.min.js
//go:embed assets/images/*
//go:embed assets/fonts/*
var embeddedAssets embed.FS

func AssetsFileSystem() (http.FileSystem, error) {
	assetsFS, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil, err
	}
	return http.FS(assetsFS), nil
}
