package example_api

import (
	"github.com/brunolkatz/goprotos7/webadmin/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type WADashboardAPi struct{}

func New() (*WADashboardAPi, error) {
	return &WADashboardAPi{}, nil
}

func (h *WADashboardAPi) Register(r chi.Router) {
	r.Get("/example", h.GetExamplePage)
}

// GetExamplePage - Renders the main example page, loading all necessary first things....
func (h *WADashboardAPi) GetExamplePage(w http.ResponseWriter, r *http.Request) {
	err := wa_server_templs.RenderPageLayout(
		w,
		r,
		"Example Page",
		ExamplePageTempl(),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}
