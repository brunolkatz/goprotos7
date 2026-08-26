package dashboard_api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/api/httpx"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	plc_runtime "github.com/brunolkatz/goprotos7/dbtool/internals/plc-runtime"
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type varHandler interface {
	GetVariables(dbNumber int32) ([]*db_models.DbVariable, error)
	GetDbNumbers(ctx context.Context) ([]uint32, error)
	GetDbVar(ctx context.Context, id int64) (*db_models.DbVariable, error)
	SetListVar(ctx context.Context, dbNumber, varId, stsId int64) (*db_models.DbVariable, error)
	SavePLCWatchList(ctx context.Context, source string, dbNumber int32, addresses []string) error
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
		r.Get("/plc-values", h.GetPLCValues)
		r.Post("/select-db", h.SelectDB)
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
	query, err := bindGetDbVarsQuery(r)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	dbVariables, err := h.varsHandler.GetVariables(query.DBNumber)
	if err != nil {
		httpx.InternalError(w, "Error fetching variables: "+err.Error())
		return
	}
	applyLiveValues(dbVariables)

	err = wa_server_templs.RenderPageLayout(
		w,
		r,
		"Database Variables",
		DbVarsTempl(dbVariables),
	)
	if err != nil {
		httpx.InternalError(w, "Error rendering page: "+err.Error())
		return
	}
}

func (h *DashboardAPi) SetDbVar(w http.ResponseWriter, r *http.Request) {
	form, err := bindSetDbVarForm(r)
	if err != nil {
		httpx.AlertError(w, r, "Error: "+err.Error())
		return
	}
	var dbVar *db_models.DbVariable

	switch form.VarType {
	case dbtool.VarTypeStatic:
		httpx.AlertError(w, r, "Error: STATIC var type update is not implemented")
		return
	case dbtool.VarTypeList:
		dbVar, err = h.varsHandler.SetListVar(r.Context(), form.DBNumber, form.VarID, form.StatusID)
		if err != nil {
			httpx.AlertError(w, r, "Error: "+err.Error())
			return
		}
	default:
		httpx.AlertError(w, r, fmt.Sprintf("Error: unsupported variable type: %s", form.VarType))
		return
	}
	if dbVar == nil {
		httpx.AlertError(w, r, "Error: Something goes wrong =/")
		return
	}
	applyLiveValues([]*db_models.DbVariable{dbVar})

	comp := DbVarTempl(dbVar)
	err = comp.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Error rendering component: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

type plcValueRow struct {
	ID      int64  `json:"id"`
	Address string `json:"address"`
	Value   string `json:"value,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (h *DashboardAPi) GetPLCValues(w http.ResponseWriter, r *http.Request) {
	query, err := bindGetDbVarsQuery(r)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	dbVariables, err := h.varsHandler.GetVariables(query.DBNumber)
	if err != nil {
		httpx.InternalError(w, "Error fetching variables: "+err.Error())
		return
	}
	rows := make([]plcValueRow, 0, len(dbVariables))
	for _, v := range dbVariables {
		row := plcValueRow{
			ID:      v.Id,
			Address: v.ToDBAddress(),
		}
		snapshot, ok := plc_runtime.GetValue(row.Address)
		if ok {
			row.Value = snapshot.Value
			row.Error = snapshot.Error
		}
		rows = append(rows, row)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func applyLiveValues(vars []*db_models.DbVariable) {
	for _, v := range vars {
		snapshot, ok := plc_runtime.GetValue(v.ToDBAddress())
		if !ok {
			continue
		}
		if snapshot.Value != "" {
			val := snapshot.Value
			v.PLCReadValue = &val
		}
		if snapshot.Error != "" {
			e := snapshot.Error
			v.PLCReadError = &e
		}
	}
}

type selectDBReq struct {
	DBNumber int32 `json:"db_number"`
}

func (h *DashboardAPi) SelectDB(w http.ResponseWriter, r *http.Request) {
	var body selectDBReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DBNumber <= 0 {
		httpx.BadRequest(w, "invalid db_number")
		return
	}
	dbVariables, err := h.varsHandler.GetVariables(body.DBNumber)
	if err != nil {
		httpx.InternalError(w, "Error fetching variables: "+err.Error())
		return
	}
	addresses := make([]string, 0, len(dbVariables))
	for _, v := range dbVariables {
		addresses = append(addresses, v.ToDBAddress())
	}
	if err := h.varsHandler.SavePLCWatchList(r.Context(), "dashboard", body.DBNumber, addresses); err != nil {
		httpx.InternalError(w, "Error saving watch list: "+err.Error())
		return
	}
	plc_runtime.SetDashboardWatchList(body.DBNumber, addresses)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"db_number": body.DBNumber,
		"count":     len(addresses),
	})
}
