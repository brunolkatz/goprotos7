package dashboard_api

import (
	"context"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type varHandler interface {
	GetVariables(dbNumber int32) ([]*db_models.DbVariable, error)
	GetDbNumbers(ctx context.Context) ([]uint32, error)
	GetDbVar(ctx context.Context, id int64) (*db_models.DbVariable, error)
	SetListVar(ctx context.Context, dbNumber, varId, stsId int64) (*db_models.DbVariable, error)
}

type DashboardAPi struct {
	varsHandler varHandler
}

func New(varsHandler varHandler) (*DashboardAPi, error) {
	return &DashboardAPi{
		varsHandler,
	}, nil
}

func (h *DashboardAPi) Register(r chi.Router) {
	r.Get("/", h.GetHomePage)
	r.Route("/dashboard", func(r chi.Router) {
		r.Get("/", h.GetHomePage) // Redirect root to /dashboard
		r.Get("/get-db-vars", h.GetDbVars)
		r.Put("/set-var-value", h.SetDbVar)
	})
}

func (h *DashboardAPi) GetHomePage(w http.ResponseWriter, r *http.Request) {

	dbNumbers, err := h.varsHandler.GetDbNumbers(r.Context())
	if err != nil {
		http.Error(w, "Error fetching database numbers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = wa_server_templs.RenderPageLayout(
		w,
		r,
		"Dashboard",
		DashboardPageTempl(dbNumbers),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

func (h *DashboardAPi) GetDbVars(w http.ResponseWriter, r *http.Request) {
	_dbNumber := r.URL.Query().Get("db-number")
	if _dbNumber == "" {
		http.Error(w, "dbNumber is required", http.StatusBadRequest)
		return
	}
	dbNumber, err := strconv.ParseInt(_dbNumber, 10, 32)

	dbVariables, err := h.varsHandler.GetVariables(int32(dbNumber))
	if err != nil {
		http.Error(w, "Error fetching variables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = wa_server_templs.RenderPageLayout(
		w,
		r,
		"Database Variables",
		DbVarsTempl(dbVariables),
	)
	if err != nil {
		http.Error(w, "Error rendering page: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *DashboardAPi) SetDbVar(w http.ResponseWriter, r *http.Request) {
	// { "db-number": 1, "var-id": 2, "t": "LIST", "def-id": 3 }
	err := r.ParseForm()
	if err != nil {
		wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Error: "+err.Error(), w, r)
		return
	}
	_dbNumber := r.Form.Get("db-number")
	if _dbNumber == "" {
		http.Error(w, "dbNumber is required", http.StatusBadRequest)
		return
	}
	dbNumber, err := strconv.ParseInt(_dbNumber, 10, 32)
	if err != nil {
		http.Error(w, "Invalid dbNumber: "+err.Error(), http.StatusBadRequest)
		return
	}
	_varId := r.Form.Get("var-id")
	if _varId == "" {
		http.Error(w, "varId is required", http.StatusBadRequest)
		return
	}
	varId, err := strconv.ParseInt(_varId, 10, 64)
	if err != nil {
		http.Error(w, "Invalid varId: "+err.Error(), http.StatusBadRequest)
		return
	}

	varType := r.Form.Get("t")
	if varType == "" {
		http.Error(w, "Variable type (t) is required", http.StatusBadRequest)
		return
	}
	if _, ok := dbtool.VarTypePara[varType]; !ok {
		http.Error(w, "Invalid variable type: "+varType, http.StatusBadRequest)
		return
	}

	var dbVar *db_models.DbVariable

	switch dbtool.VarTypePara[varType] {
	case dbtool.VarTypeStatic:
		// TODO: Create the logic to handle static variable types
	case dbtool.VarTypeList:
		_stsId := r.Form.Get("sts-id")
		if _stsId == "" {
			http.Error(w, "defId is required", http.StatusBadRequest)
			return
		}
		stsId, err := strconv.ParseInt(_stsId, 10, 64)
		if err != nil {
			http.Error(w, "Invalid defId: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Here you would typically call a method to set the variable value in the database
		dbVar, err = h.varsHandler.SetListVar(r.Context(), dbNumber, varId, stsId) // Replace 1 with the actual variable ID
		if err != nil {
			wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Error: "+err.Error(), w, r)
			return
		}
	}
	if dbVar == nil {
		wa_server_templs.RenderAlertMSG(wa_server_templs.RT_Error, "Error: Something goes wrong =/", w, r)
		return
	}

	comp := DbVarTempl(dbVar)
	err = comp.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Error rendering component: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
