package create_var_api

import (
	"context"
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/api/httpx"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type varsHandler interface {
	CreateVariable(ctx context.Context, newVar *dbtool.CreateVarRequest) (*db_models.DbVariable, error)
	GetDbNumbers(ctx context.Context) ([]uint32, error)
}

type CreateVarAPi struct {
	varsHandler varsHandler
}

func New(varsHandler varsHandler) (*CreateVarAPi, error) {
	if varsHandler == nil {
		return nil, fmt.Errorf("varsHandler is nil")
	}
	return &CreateVarAPi{
		varsHandler,
	}, nil
}

func (h *CreateVarAPi) Register(r chi.Router) {
	r.Route("/vars", func(r chi.Router) {
		r.Get("/", h.GetCreateVarPage)
		r.Post("/create-var", h.CreateNewVar)
		r.Get("/import-csv", h.GetImportCSVPage)
		r.Get("/import-csv/example", h.DownloadImportCSVExample)
		r.Post("/import-csv/upload", h.ImportCSVVariables)
	})
}

// GetCreateVarPage renders the create variable page.
func (h *CreateVarAPi) GetCreateVarPage(w http.ResponseWriter, r *http.Request) {
	dbNumbers, err := h.varsHandler.GetDbNumbers(r.Context())
	if err != nil {
		httpx.AlertError(w, r, "Error loading DB numbers: "+err.Error())
		return
	}

	err = wa_server_templs.RenderPageLayout(
		w,
		r,
		"Create Variable",
		CreateVarPageTempl(dbNumbers),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

func (h *CreateVarAPi) CreateNewVar(w http.ResponseWriter, r *http.Request) {

	req, err := bindCreateVarRequest(r)
	if err != nil {
		httpx.AlertError(w, r, "Error parsing form: "+err.Error())
		return
	}

	_, err = h.varsHandler.CreateVariable(r.Context(), req)
	if err != nil {
		httpx.AlertError(w, r, "Error creating variable: "+err.Error())
		return
	}
	httpx.AlertSuccess(w, r, "Variable created successfully")
}

func (h *CreateVarAPi) GetImportCSVPage(w http.ResponseWriter, r *http.Request) {
	err := wa_server_templs.RenderPageLayout(
		w,
		r,
		"Import Variables CSV",
		ImportCSVPageTempl(),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
