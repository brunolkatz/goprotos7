package connect_plc_api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	plc_runtime "github.com/brunolkatz/goprotos7/dbtool/internals/plc-runtime"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/robinson/gos7"
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
		r.Get("/ws", h.HandleWS)
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
	items := normalizeVariables(body.Addresses)
	if err := h.varsHandler.SavePLCWatchList(r.Context(), body.Source, body.DBNumber, items); err != nil {
		http.Error(w, "Error saving watch list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"source":    body.Source,
		"db_number": body.DBNumber,
		"count":     len(items),
	})
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsCommand struct {
	Type      string   `json:"type"`
	Address   string   `json:"address"`
	Port      int      `json:"port"`
	Rack      int      `json:"rack"`
	Slot      int      `json:"slot"`
	PollMS    int      `json:"poll_ms"`
	Variables []string `json:"variables"`
}

type valueRow struct {
	Address string `json:"address"`
	Value   string `json:"value,omitempty"`
	Error   string `json:"error,omitempty"`
}

type wsEvent struct {
	Type      string          `json:"type"`
	Message   string          `json:"message,omitempty"`
	Status    string          `json:"status,omitempty"`
	CPUInfo   *gos7.S7CpuInfo `json:"cpu_info,omitempty"`
	Values    []valueRow      `json:"values,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
}

type plcSession struct {
	ws *websocket.Conn

	writeMu sync.Mutex

	handler *gos7.TCPClientHandler
	client  gos7.Client

	variables []string
	pollMS    int

	stopPoll context.CancelFunc

	saveConnectList func(items []string)
}

func (h *ConnectPLCAPI) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s := &plcSession{
		ws: conn,
		saveConnectList: func(items []string) {
			_ = h.varsHandler.SavePLCWatchList(context.Background(), "connect", 0, items)
		},
	}
	defer s.close()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var cmd wsCommand
		if err = json.Unmarshal(payload, &cmd); err != nil {
			_ = s.send(wsEvent{Type: "error", Message: "invalid websocket payload"})
			continue
		}
		if err = s.handleCommand(r.Context(), cmd); err != nil {
			_ = s.send(wsEvent{Type: "error", Message: err.Error()})
		}
	}
}

func (s *plcSession) handleCommand(ctx context.Context, cmd wsCommand) error {
	switch cmd.Type {
	case "connect":
		return s.connectAndStart(ctx, cmd)
	case "disconnect":
		s.disconnect()
		return s.send(wsEvent{Type: "disconnected", Message: "disconnected"})
	case "read_once":
		return s.readOnce()
	case "get_status":
		return s.sendStatus()
	case "get_cpu_info":
		return s.sendCPUInfo()
	case "set_variables":
		s.variables = normalizeVariables(cmd.Variables)
		if s.saveConnectList != nil {
			s.saveConnectList(s.variables)
		}
		return s.send(wsEvent{Type: "info", Message: fmt.Sprintf("watching %d variable(s)", len(s.variables))})
	default:
		return fmt.Errorf("unsupported command: %s", cmd.Type)
	}
}

func (s *plcSession) connectAndStart(ctx context.Context, cmd wsCommand) error {
	if strings.TrimSpace(cmd.Address) == "" {
		return fmt.Errorf("address is required")
	}
	if cmd.PollMS <= 0 {
		cmd.PollMS = 1000
	}

	s.disconnect()

	address := strings.TrimSpace(cmd.Address)
	if cmd.Port > 0 && !strings.Contains(address, ":") {
		address = fmt.Sprintf("%s:%d", address, cmd.Port)
	}

	handler := gos7.NewTCPClientHandler(address, cmd.Rack, cmd.Slot)
	handler.Timeout = 10 * time.Second
	handler.IdleTimeout = 60 * time.Second
	if err := handler.Connect(); err != nil {
		plc_runtime.SetDisconnected(err.Error())
		return fmt.Errorf("connect failed: %w", err)
	}

	s.handler = handler
	s.client = gos7.NewClient(handler)
	s.variables = normalizeVariables(cmd.Variables)
	if s.saveConnectList != nil {
		s.saveConnectList(s.variables)
	}
	s.pollMS = cmd.PollMS
	plc_runtime.SetConnected(address)

	if err := s.send(wsEvent{Type: "connected", Message: "connection established"}); err != nil {
		return err
	}
	_ = s.sendCPUInfo()
	_ = s.sendStatus()

	pollCtx, cancel := context.WithCancel(ctx)
	s.stopPoll = cancel
	go s.pollLoop(pollCtx)
	return nil
}

func (s *plcSession) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.pollMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.readOnce()
		}
	}
}

func (s *plcSession) readOnce() error {
	if s.client == nil {
		return fmt.Errorf("not connected")
	}
	allVariables := mergeVariables(s.variables, plc_runtime.GetActiveDashboardWatchList())
	values := make([]valueRow, 0, len(allVariables))
	for _, addr := range allVariables {
		buff := make([]byte, 256)
		v, err := s.client.Read(addr, buff)
		if err != nil {
			plc_runtime.SetValue(addr, "", err.Error())
			values = append(values, valueRow{Address: addr, Error: err.Error()})
			continue
		}
		normalized := fmt.Sprintf("%v", normalizeValue(addr, v, buff))
		plc_runtime.SetValue(addr, normalized, "")

		values = append(values, valueRow{
			Address: addr,
			Value:   normalized,
		})
	}
	status := ""
	if sts, err := s.client.PLCGetStatus(); err == nil {
		status = mapPLCStatus(sts)
		plc_runtime.SetPLCStatus(status)
	}

	return s.send(wsEvent{
		Type:      "values",
		Status:    status,
		Values:    values,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (s *plcSession) sendStatus() error {
	if s.client == nil {
		return fmt.Errorf("not connected")
	}
	sts, err := s.client.PLCGetStatus()
	if err != nil {
		return fmt.Errorf("status read failed: %w", err)
	}
	status := mapPLCStatus(sts)
	plc_runtime.SetPLCStatus(status)
	return s.send(wsEvent{Type: "status", Status: status})
}

func (s *plcSession) sendCPUInfo() error {
	if s.client == nil {
		return fmt.Errorf("not connected")
	}
	info, err := s.client.GetCPUInfo()
	if err != nil {
		return fmt.Errorf("cpu info read failed: %w", err)
	}
	return s.send(wsEvent{Type: "cpu_info", CPUInfo: &info})
}

func (s *plcSession) disconnect() {
	if s.stopPoll != nil {
		s.stopPoll()
		s.stopPoll = nil
	}
	if s.handler != nil {
		_ = s.handler.Close()
	}
	s.handler = nil
	s.client = nil
	plc_runtime.SetDisconnected("")
}

func (s *plcSession) close() {
	s.disconnect()
	_ = s.ws.Close()
}

func (s *plcSession) send(msg wsEvent) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.ws.WriteJSON(msg)
}

func normalizeVariables(items []string) []string {
	ret := make([]string, 0, len(items))
	for _, i := range items {
		v := strings.ToUpper(strings.TrimSpace(i))
		if v != "" {
			ret = append(ret, v)
		}
	}
	slices.Sort(ret)
	ret = slices.Compact(ret)
	return ret
}

func mergeVariables(a, b []string) []string {
	all := make([]string, 0, len(a)+len(b))
	all = append(all, a...)
	all = append(all, b...)
	return normalizeVariables(all)
}

var realAddr = regexp.MustCompile(`(?i)\.DBD[0-9]+$`)

func normalizeValue(address string, value any, raw []byte) any {
	if realAddr.MatchString(strings.TrimSpace(address)) {
		var h gos7.Helper
		return h.GetRealAt(raw, 0)
	}
	return value
}

func mapPLCStatus(s int) string {
	switch s {
	case 8:
		return "RUN"
	case 4:
		return "STOP"
	default:
		return "UNKNOWN"
	}
}
