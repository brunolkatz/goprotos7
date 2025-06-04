package dashboard_api

import (
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type WADashboardAPi struct{}

func New() (*WADashboardAPi, error) {
	return &WADashboardAPi{}, nil
}

func (h *WADashboardAPi) Register(r chi.Router) {
	r.Get("/", h.GetHomePage)
	r.Get("/dashboard", h.GetHomePage)
}

func (h *WADashboardAPi) GetHomePage(w http.ResponseWriter, r *http.Request) {
	err := wa_server_templs.RenderPageLayout(
		w,
		r,
		"Dashboard",
		DashboardPageTempl(),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}
