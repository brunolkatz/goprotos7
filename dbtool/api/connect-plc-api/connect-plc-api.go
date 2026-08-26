package connect_plc_api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	plc_runtime "github.com/brunolkatz/goprotos7/dbtool/internals/plc-runtime"
	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
)

type varsHandler interface {
	GetDbNumbers(ctx context.Context) ([]uint32, error)
	GetVariables(dbNumber int32) ([]*db_models.DbVariable, error)
	GetPLCWatchList(ctx context.Context, source string, dbNumber int32) ([]string, error)
	SavePLCWatchList(ctx context.Context, source string, dbNumber int32, addresses []string) error
}

type ConnectPLCAPI struct {
	varsHandler varsHandler
}

func New(varsHandler varsHandler) (*ConnectPLCAPI, error) {
	return &ConnectPLCAPI{varsHandler: varsHandler}, nil
}

func (h *ConnectPLCAPI) Register(r chi.Router) {
	r.Route("/connect-plc", func(r chi.Router) {
		r.Get("/", h.GetPage)
		r.Get("/variables", h.GetVariables)
		r.Get("/runtime-status", h.GetRuntimeStatus)
		r.Get("/watch-list", h.GetWatchList)
		r.Put("/watch-list", h.SaveWatchList)
	})
}

func (h *ConnectPLCAPI) GetPage(w http.ResponseWriter, r *http.Request) {
	dbNumbers, err := h.varsHandler.GetDbNumbers(r.Context())
	if err != nil {
		http.Error(w, "Error fetching database numbers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	connectList, err := h.varsHandler.GetPLCWatchList(r.Context(), "connect", 0)
	if err != nil {
		http.Error(w, "Error fetching watch list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	err = wa_server_templs.RenderPageLayout(w, r, "Connect to PLC", ConnectPLCPageTempl(dbNumbers, connectList))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type dbVariableRow struct {
	ID          int64  `json:"id"`
	DBNumber    uint32 `json:"db_number"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DataType    string `json:"data_type"`
	Address     string `json:"address"`
}

func (h *ConnectPLCAPI) GetVariables(w http.ResponseWriter, r *http.Request) {
	dbNumberText := strings.TrimSpace(r.URL.Query().Get("db-number"))
	if dbNumberText == "" {
		http.Error(w, "missing db-number query param", http.StatusBadRequest)
		return
	}
	dbNumberValue, err := strconv.ParseInt(dbNumberText, 10, 32)
	if err != nil || dbNumberValue <= 0 {
		http.Error(w, "invalid db-number query param", http.StatusBadRequest)
		return
	}
	dbVars, err := h.varsHandler.GetVariables(int32(dbNumberValue))
	if err != nil {
		http.Error(w, "Error fetching variables: "+err.Error(), http.StatusInternalServerError)
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	rows := make([]dbVariableRow, 0, len(dbVars))
	for _, v := range dbVars {
		row := dbVariableRow{
			ID:          v.Id,
			DBNumber:    uint32(v.DbNumber),
			Name:        v.Name,
			Description: v.Description,
			DataType:    fmt.Sprintf("%v", v.DataType),
			Address:     v.ToDBAddress(),
		}
		if query != "" {
			text := strings.ToLower(fmt.Sprintf("%s %s %s %s", row.Name, row.Description, row.DataType, row.Address))
			if !strings.Contains(text, query) {
				continue
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Address == rows[j].Address {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].Address < rows[j].Address
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func (h *ConnectPLCAPI) GetRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(plc_runtime.GetRuntimeStatus())
}

func (h *ConnectPLCAPI) GetWatchList(w http.ResponseWriter, r *http.Request) {
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if source == "" {
		source = "connect"
	}
	dbNumber := int32(0)
	dbText := strings.TrimSpace(r.URL.Query().Get("db-number"))
	if dbText != "" {
		n, err := strconv.ParseInt(dbText, 10, 32)
		if err != nil {
			http.Error(w, "invalid db-number query param", http.StatusBadRequest)
			return
		}
		dbNumber = int32(n)
	}
	items, err := h.varsHandler.GetPLCWatchList(r.Context(), source, dbNumber)
	if err != nil {
		http.Error(w, "Error fetching watch list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

type saveWatchListReq struct {
	Source    string   `json:"source"`
	DBNumber  int32    `json:"db_number"`
	Addresses []string `json:"addresses"`
}

func (h *ConnectPLCAPI) SaveWatchList(w http.ResponseWriter, r *http.Request) {
	var body saveWatchListReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Source) == "" {
		body.Source = "connect"
	}
	if err := h.varsHandler.SavePLCWatchList(r.Context(), body.Source, body.DBNumber, body.Addresses); err != nil {
		http.Error(w, "Error saving watch list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"source":    body.Source,
		"db_number": body.DBNumber,
		"count":     len(body.Addresses),
	})
}
