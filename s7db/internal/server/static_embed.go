package server

import "embed"

//go:embed static/*
var staticFS embed.FS
