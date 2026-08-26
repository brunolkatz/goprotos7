package heartbeats_api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool/api/httpx"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	ws_handler "github.com/brunolkatz/goprotos7/dbtool/handlers/ws-handler"
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
)

type varsHandler interface {
	GetDbNumbers(ctx context.Context) ([]uint32, error)
	GetVariables(dbNumber int32) ([]*db_models.DbVariable, error)
	CreateHeartbeat(ctx context.Context, hb *db_models.HeartbeatRegistration) error
	ListHeartbeats(ctx context.Context) ([]*db_models.HeartbeatRegistration, error)
	GetHeartbeatByID(ctx context.Context, id int64) (*db_models.HeartbeatRegistration, error)
	DeleteHeartbeatByID(ctx context.Context, id int64) error
}

type HeartbeatsAPI struct {
	varsHandler varsHandler
}

func New(varsHandler varsHandler) (*HeartbeatsAPI, error) {
	if varsHandler == nil {
		return nil, fmt.Errorf("varsHandler is nil")
	}
	return &HeartbeatsAPI{varsHandler: varsHandler}, nil
}

func (h *HeartbeatsAPI) Register(r chi.Router) {
	r.Route("/heartbeats", func(r chi.Router) {
		r.Get("/", h.GetPage)
		r.Get("/bool-vars", h.GetBoolVariables)
		r.Get("/list", h.GetHeartbeatList)
		r.Get("/card/{id}", h.GetHeartbeatCard)
		r.Post("/delete/{id}", h.DeleteHeartbeat)
		r.Post("/register", h.RegisterHeartbeat)
	})
}

func (h *HeartbeatsAPI) GetPage(w http.ResponseWriter, r *http.Request) {
	dbNumbers, err := h.varsHandler.GetDbNumbers(r.Context())
	if err != nil {
		http.Error(w, "Error loading DB numbers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	heartbeats, err := h.varsHandler.ListHeartbeats(r.Context())
	if err != nil {
		http.Error(w, "Error loading heartbeats: "+err.Error(), http.StatusInternalServerError)
		return
	}
	err = wa_server_templs.RenderPageLayout(w, r, "Register Heartbeats", HeartbeatsPageTempl(dbNumbers, heartbeats))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type boolVarRow struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (h *HeartbeatsAPI) GetBoolVariables(w http.ResponseWriter, r *http.Request) {
	dbText := strings.TrimSpace(r.URL.Query().Get("db-number"))
	dbNumber, err := strconv.ParseInt(dbText, 10, 32)
	if err != nil || dbNumber <= 0 {
		httpx.BadRequest(w, "invalid db-number")
		return
	}
	vars, err := h.varsHandler.GetVariables(int32(dbNumber))
	if err != nil {
		httpx.InternalError(w, "Error loading variables: "+err.Error())
		return
	}
	out := make([]boolVarRow, 0)
	for _, v := range vars {
		if v.DataType != goprotos7.BOOL {
			continue
		}
		out = append(out, boolVarRow{ID: v.Id, Name: v.Name, Address: v.ToDBAddress()})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *HeartbeatsAPI) GetHeartbeatList(w http.ResponseWriter, r *http.Request) {
	heartbeats, err := h.varsHandler.ListHeartbeats(r.Context())
	if err != nil {
		httpx.InternalError(w, "Error loading heartbeats: "+err.Error())
		return
	}
	comp := HeartbeatListTempl(heartbeats)
	if err := comp.Render(r.Context(), w); err != nil {
		httpx.InternalError(w, "Error rendering list: "+err.Error())
	}
}

func (h *HeartbeatsAPI) RegisterHeartbeat(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpx.AlertError(w, r, "Error parsing form: "+err.Error())
		return
	}
	dbNumber, err := strconv.ParseInt(strings.TrimSpace(r.Form.Get("db-number")), 10, 64)
	if err != nil || dbNumber <= 0 {
		httpx.AlertError(w, r, "Invalid DB number")
		return
	}
	address := strings.ToUpper(strings.TrimSpace(r.Form.Get("address")))
	name := strings.TrimSpace(r.Form.Get("variable-name"))
	if address == "" || name == "" {
		httpx.AlertError(w, r, "Address and variable name are required")
		return
	}
	heartbeatType := strings.TrimSpace(r.Form.Get("heartbeat-type"))
	if heartbeatType != string(db_models.HeartbeatTypeWhenTrueSetFalse) && heartbeatType != string(db_models.HeartbeatTypeWhenFalseSetTrue) {
		httpx.AlertError(w, r, "Invalid heartbeat type")
		return
	}
	alarmSeconds := int64(5)
	if raw := strings.TrimSpace(r.Form.Get("alarm-seconds")); raw != "" {
		v, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || v <= 0 {
			httpx.AlertError(w, r, "Alarm must be a positive number")
			return
		}
		alarmSeconds = v
	}
	hb := &db_models.HeartbeatRegistration{
		DBNumber:       dbNumber,
		VariableName:   name,
		Address:        address,
		StartOnConnect: r.Form.Get("start-on-connect") == "on",
		Enabled:        true,
		HeartbeatType:  db_models.HeartbeatType(heartbeatType),
		AlarmSeconds:   alarmSeconds,
	}
	if err := h.varsHandler.CreateHeartbeat(r.Context(), hb); err != nil {
		httpx.AlertError(w, r, "Error creating heartbeat: "+err.Error())
		return
	}
	httpx.AlertSuccess(w, r, "Heartbeat registered successfully")
}

func (h *HeartbeatsAPI) GetHeartbeatCard(w http.ResponseWriter, r *http.Request) {
	idText := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(strings.TrimSpace(idText), 10, 64)
	if err != nil || id <= 0 {
		httpx.BadRequest(w, "invalid heartbeat id")
		return
	}
	hb, err := h.varsHandler.GetHeartbeatByID(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, "Error loading heartbeat: "+err.Error())
		return
	}
	comp := HeartbeatCardTempl(hb)
	if err := comp.Render(r.Context(), w); err != nil {
		httpx.InternalError(w, "Error rendering heartbeat card: "+err.Error())
	}
}

func (h *HeartbeatsAPI) DeleteHeartbeat(w http.ResponseWriter, r *http.Request) {
	idText := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(strings.TrimSpace(idText), 10, 64)
	if err != nil || id <= 0 {
		httpx.BadRequest(w, "invalid heartbeat id")
		return
	}
	if ws := ws_handler.Instance(); ws != nil {
		ws.StopHeartbeat(id)
	}
	if err := h.varsHandler.DeleteHeartbeatByID(r.Context(), id); err != nil {
		httpx.InternalError(w, "Error deleting heartbeat: "+err.Error())
		return
	}
	heartbeats, err := h.varsHandler.ListHeartbeats(r.Context())
	if err != nil {
		httpx.InternalError(w, "Error loading heartbeats: "+err.Error())
		return
	}
	comp := HeartbeatListTempl(heartbeats)
	if err := comp.Render(r.Context(), w); err != nil {
		httpx.InternalError(w, "Error rendering heartbeat list: "+err.Error())
	}
}
