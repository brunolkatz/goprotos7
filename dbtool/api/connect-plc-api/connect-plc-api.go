package connect_plc_api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	plc_runtime "github.com/brunolkatz/goprotos7/dbtool/internals/plc-runtime"

	"github.com/brunolkatz/goprotos7/dbtool/internals/wa-server-templs"
	"github.com/get-notify/gos7"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type varsHandler interface {
	GetDbNumbers(ctx context.Context) ([]uint32, error)
	GetVariables(dbNumber int32) ([]*db_models.DbVariable, error)
	GetPLCWatchList(ctx context.Context, source string, dbNumber int32) ([]string, error)
	SavePLCWatchList(ctx context.Context, source string, dbNumber int32, addresses []string) error
	ListActiveHeartbeats(ctx context.Context) ([]*db_models.HeartbeatRegistration, error)
	UpdateHeartbeatRuntime(ctx context.Context, id int64, lastValue *bool, lastCheckAt, lastChangeAt *time.Time, isFailing bool, lastError string) error
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
	varsHandler     varsHandler
}

func (h *ConnectPLCAPI) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	plc_runtime.SetWSState("CONNECTED")
	s := &plcSession{
		ws: conn,
		saveConnectList: func(items []string) {
			_ = h.varsHandler.SavePLCWatchList(context.Background(), "connect", 0, items)
		},
		varsHandler: h.varsHandler,
	}
	defer s.close()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			plc_runtime.SetWSState("DISCONNECTED")
			return
		}
		plc_runtime.MarkWSMessage()
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
	plc_runtime.SetConnecting()

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
	plc_runtime.SetClient(s.client)
	plc_runtime.SetConnectionConfig(address, cmd.Rack, cmd.Slot, cmd.PollMS)
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
	readValues, err := plc_runtime.ReadValues(allVariables)
	if err != nil {
		return err
	}
	for _, item := range readValues {
		values = append(values, valueRow{
			Address: item.Address,
			Value:   item.Value,
			Error:   item.Error,
		})
	}
	status := ""
	if sts, err := s.client.PLCGetStatus(); err == nil {
		status = mapPLCStatus(sts)
		plc_runtime.SetPLCStatus(status)
	}
	_ = s.runHeartbeats()

	return s.send(wsEvent{
		Type:      "values",
		Status:    status,
		Values:    values,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (s *plcSession) runHeartbeats() error {
	if s.client == nil || s.varsHandler == nil {
		return nil
	}
	heartbeats, err := s.varsHandler.ListActiveHeartbeats(context.Background())
	if err != nil {
		return err
	}
	now := time.Now()
	for _, hb := range heartbeats {
		if hb.LastCheckAt != nil && hb.AlarmSeconds > 0 {
			if now.Sub(*hb.LastCheckAt) < time.Duration(hb.AlarmSeconds)*time.Second {
				continue
			}
		}
		value, readErr := readBoolAtAddress(s.client, hb.Address)
		if readErr != nil {
			_ = s.varsHandler.UpdateHeartbeatRuntime(context.Background(), hb.ID, hb.LastValue, &now, hb.LastChangeAt, true, readErr.Error())
			continue
		}

		lastChangeAt := hb.LastChangeAt
		if hb.LastValue == nil || *hb.LastValue != value {
			t := now
			lastChangeAt = &t
		}

		switch hb.HeartbeatType {
		case db_models.HeartbeatTypeWhenTrueSetFalse:
			if value {
				_ = writeBoolAtAddress(s.client, hb.Address, false)
			}
		case db_models.HeartbeatTypeWhenFalseSetTrue:
			if !value {
				_ = writeBoolAtAddress(s.client, hb.Address, true)
			}
		}

		isFailing := false
		if lastChangeAt != nil && now.Sub(*lastChangeAt) > time.Duration(hb.AlarmSeconds)*time.Second {
			isFailing = true
		}
		v := value
		_ = s.varsHandler.UpdateHeartbeatRuntime(context.Background(), hb.ID, &v, &now, lastChangeAt, isFailing, "")
	}
	return nil
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
	plc_runtime.SetCPUInfo(info)
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
	plc_runtime.ClearClient()
	plc_runtime.SetDisconnected("")
}

func (s *plcSession) close() {
	s.disconnect()
	plc_runtime.SetWSState("DISCONNECTED")
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

func readBoolAtAddress(client gos7.Client, address string) (bool, error) {
	meta, err := parseBoolAddress(address)
	if err != nil {
		return false, err
	}
	buf := make([]byte, 1)
	if err := client.AGReadDB(meta.dbNumber, meta.byteOffset, 1, buf); err != nil {
		return false, err
	}
	mask := byte(1 << meta.bitOffset)
	return (buf[0] & mask) != 0, nil
}

func writeBoolAtAddress(client gos7.Client, address string, value bool) error {
	meta, err := parseBoolAddress(address)
	if err != nil {
		return err
	}
	buf := make([]byte, 1)
	if err := client.AGReadDB(meta.dbNumber, meta.byteOffset, 1, buf); err != nil {
		return err
	}
	mask := byte(1 << meta.bitOffset)
	if value {
		buf[0] = buf[0] | mask
	} else {
		buf[0] = buf[0] &^ mask
	}
	return client.AGWriteDB(meta.dbNumber, meta.byteOffset, 1, buf)
}

type boolAddressMeta struct {
	dbNumber   int
	byteOffset int
	bitOffset  int
}

func parseBoolAddress(address string) (*boolAddressMeta, error) {
	addr := strings.ToUpper(strings.TrimSpace(address))
	if !strings.HasPrefix(addr, "DB") || !strings.Contains(addr, ".DBX") {
		return nil, fmt.Errorf("invalid BOOL address: %s", address)
	}
	parts := strings.SplitN(addr, ".DBX", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid BOOL address: %s", address)
	}
	dbNumber, err := strconv.Atoi(strings.TrimPrefix(parts[0], "DB"))
	if err != nil {
		return nil, fmt.Errorf("invalid BOOL DB number: %s", address)
	}
	byteBit := strings.SplitN(parts[1], ".", 2)
	if len(byteBit) != 2 {
		return nil, fmt.Errorf("invalid BOOL byte/bit: %s", address)
	}
	byteOffset, err := strconv.Atoi(byteBit[0])
	if err != nil {
		return nil, fmt.Errorf("invalid BOOL byte offset: %s", address)
	}
	bitOffset, err := strconv.Atoi(byteBit[1])
	if err != nil || bitOffset < 0 || bitOffset > 7 {
		return nil, fmt.Errorf("invalid BOOL bit offset: %s", address)
	}
	return &boolAddressMeta{dbNumber: dbNumber, byteOffset: byteOffset, bitOffset: bitOffset}, nil
}
