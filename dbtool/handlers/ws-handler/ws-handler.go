package ws_handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	plc_runtime "github.com/brunolkatz/goprotos7/dbtool/internals/plc-runtime"
	"github.com/get-notify/gos7"
	"github.com/gorilla/websocket"
)

type VarsHandler interface {
	SavePLCWatchList(ctx context.Context, source string, dbNumber int32, addresses []string) error
	ListActiveHeartbeats(ctx context.Context) ([]*db_models.HeartbeatRegistration, error)
	UpdateHeartbeatRuntime(ctx context.Context, id int64, lastValue *bool, lastCheckAt, lastChangeAt *time.Time, isFailing bool, lastError string) error
}

type WSHandler struct {
	server      *http.Server
	varsHandler VarsHandler

	upgrader websocket.Upgrader
	connsMu  sync.RWMutex
	conns    map[*websocket.Conn]struct{}

	plcMu     sync.Mutex
	handler   *gos7.TCPClientHandler
	client    gos7.Client
	variables []string
	pollMS    int
	stopPoll  context.CancelFunc
	hbCancels map[int64]context.CancelFunc
}

var wsHandlerInstance *WSHandler

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

func New(varsHandler VarsHandler, addr string) (*WSHandler, error) {
	h := &WSHandler{
		varsHandler: varsHandler,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		conns:     make(map[*websocket.Conn]struct{}),
		pollMS:    1000,
		hbCancels: map[int64]context.CancelFunc{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.handleWS)
	h.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	wsHandlerInstance = h
	return h, nil
}

func Instance() *WSHandler {
	return wsHandlerInstance
}

func (h *WSHandler) ServeForErrGroup() func() error {
	return func() error {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

func (h *WSHandler) Stop(ctx context.Context) error {
	h.disconnect("")
	return h.server.Shutdown(ctx)
}

func (h *WSHandler) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	plc_runtime.SetWSState("CONNECTED")
	h.addConn(conn)
	defer func() {
		h.removeConn(conn)
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			plc_runtime.SetWSState("DISCONNECTED")
			return
		}
		plc_runtime.MarkWSMessage()
		var cmd wsCommand
		if err := json.Unmarshal(payload, &cmd); err != nil {
			_ = conn.WriteJSON(wsEvent{Type: "error", Message: "invalid websocket payload"})
			continue
		}
		if err := h.handleCommand(r.Context(), cmd); err != nil {
			_ = conn.WriteJSON(wsEvent{Type: "error", Message: err.Error()})
		}
	}
}

func (h *WSHandler) addConn(conn *websocket.Conn) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	h.conns[conn] = struct{}{}
}

func (h *WSHandler) removeConn(conn *websocket.Conn) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	delete(h.conns, conn)
}

func (h *WSHandler) broadcast(evt wsEvent) {
	h.connsMu.RLock()
	defer h.connsMu.RUnlock()
	for c := range h.conns {
		_ = c.WriteJSON(evt)
	}
}

func (h *WSHandler) handleCommand(ctx context.Context, cmd wsCommand) error {
	switch cmd.Type {
	case "connect":
		return h.connectAndStart(ctx, cmd)
	case "disconnect":
		h.disconnect("")
		h.broadcast(wsEvent{Type: "disconnected", Message: "disconnected"})
		return nil
	case "read_once":
		return h.readOnce()
	case "get_status":
		return h.sendStatus()
	case "get_cpu_info":
		return h.sendCPUInfo()
	case "set_variables":
		h.plcMu.Lock()
		h.variables = normalizeVariables(cmd.Variables)
		h.plcMu.Unlock()
		_ = h.varsHandler.SavePLCWatchList(context.Background(), "connect", 0, h.variables)
		h.broadcast(wsEvent{Type: "info", Message: fmt.Sprintf("watching %d variable(s)", len(h.variables))})
		return nil
	default:
		return fmt.Errorf("unsupported command: %s", cmd.Type)
	}
}

func (h *WSHandler) connectAndStart(ctx context.Context, cmd wsCommand) error {
	if strings.TrimSpace(cmd.Address) == "" {
		return fmt.Errorf("address is required")
	}
	if cmd.PollMS <= 0 {
		cmd.PollMS = 1000
	}
	h.disconnect("")
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

	h.plcMu.Lock()
	h.handler = handler
	h.client = gos7.NewClient(handler)
	h.pollMS = cmd.PollMS
	h.variables = normalizeVariables(cmd.Variables)
	h.plcMu.Unlock()
	plc_runtime.SetClient(h.client)
	plc_runtime.SetConnectionConfig(address, cmd.Rack, cmd.Slot, cmd.PollMS)
	plc_runtime.SetConnected(address)
	_ = h.varsHandler.SavePLCWatchList(context.Background(), "connect", 0, h.variables)

	h.broadcast(wsEvent{Type: "connected", Message: "connection established"})
	_ = h.sendCPUInfo()
	_ = h.sendStatus()

	pollCtx, cancel := context.WithCancel(ctx)
	h.plcMu.Lock()
	h.stopPoll = cancel
	h.plcMu.Unlock()
	go h.pollLoop(pollCtx)
	h.startHeartbeatWorkers()
	return nil
}

func (h *WSHandler) disconnect(errText string) {
	h.plcMu.Lock()
	defer h.plcMu.Unlock()
	if h.stopPoll != nil {
		h.stopPoll()
		h.stopPoll = nil
	}
	for _, cancel := range h.hbCancels {
		cancel()
	}
	h.hbCancels = map[int64]context.CancelFunc{}
	if h.handler != nil {
		_ = h.handler.Close()
	}
	h.handler = nil
	h.client = nil
	plc_runtime.ClearClient()
	plc_runtime.SetDisconnected(errText)
}

func (h *WSHandler) pollLoop(ctx context.Context) {
	h.plcMu.Lock()
	interval := h.pollMS
	h.plcMu.Unlock()
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = h.readOnce()
		}
	}
}

func (h *WSHandler) readOnce() error {
	h.plcMu.Lock()
	client := h.client
	vars := append([]string(nil), h.variables...)
	h.plcMu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	all := mergeVariables(vars, plc_runtime.GetActiveDashboardWatchList())
	readValues, err := plc_runtime.ReadValues(all)
	if err != nil {
		return err
	}
	values := make([]valueRow, 0, len(readValues))
	for _, item := range readValues {
		values = append(values, valueRow{Address: item.Address, Value: item.Value, Error: item.Error})
	}
	status := ""
	if sts, err := client.PLCGetStatus(); err == nil {
		status = mapPLCStatus(sts)
		plc_runtime.SetPLCStatus(status)
	}
	h.broadcast(wsEvent{
		Type:      "values",
		Status:    status,
		Values:    values,
		Timestamp: time.Now().Format(time.RFC3339),
	})
	return nil
}

func (h *WSHandler) sendStatus() error {
	h.plcMu.Lock()
	client := h.client
	h.plcMu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	sts, err := client.PLCGetStatus()
	if err != nil {
		return fmt.Errorf("status read failed: %w", err)
	}
	status := mapPLCStatus(sts)
	plc_runtime.SetPLCStatus(status)
	h.broadcast(wsEvent{Type: "status", Status: status})
	return nil
}

func (h *WSHandler) sendCPUInfo() error {
	h.plcMu.Lock()
	client := h.client
	h.plcMu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	info, err := client.GetCPUInfo()
	if err != nil {
		return fmt.Errorf("cpu info read failed: %w", err)
	}
	plc_runtime.SetCPUInfo(info)
	h.broadcast(wsEvent{Type: "cpu_info", CPUInfo: &info})
	return nil
}

func (h *WSHandler) startHeartbeatWorkers() {
	for _, cancel := range h.hbCancels {
		cancel()
	}
	h.hbCancels = map[int64]context.CancelFunc{}
	heartbeats, err := h.varsHandler.ListActiveHeartbeats(context.Background())
	if err != nil {
		return
	}
	h.plcMu.Lock()
	client := h.client
	h.plcMu.Unlock()
	if client == nil {
		return
	}
	for _, hb := range heartbeats {
		interval := hb.AlarmSeconds
		if interval <= 0 {
			interval = 5
		}
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		h.hbCancels[hb.ID] = cancel
		go func(hb *db_models.HeartbeatRegistration, tk *time.Ticker, c context.Context) {
			defer tk.Stop()
			h.runHeartbeatCheck(c, hb)
			for {
				select {
				case <-c.Done():
					return
				case <-tk.C:
					h.runHeartbeatCheck(c, hb)
				}
			}
		}(hb, ticker, ctx)
	}
}

func (h *WSHandler) StopHeartbeat(id int64) {
	h.plcMu.Lock()
	defer h.plcMu.Unlock()
	cancel, ok := h.hbCancels[id]
	if !ok {
		return
	}
	cancel()
	delete(h.hbCancels, id)
}

func (h *WSHandler) runHeartbeatCheck(ctx context.Context, hb *db_models.HeartbeatRegistration) {
	h.plcMu.Lock()
	client := h.client
	h.plcMu.Unlock()
	if client == nil || hb == nil {
		return
	}
	now := time.Now()
	value, readErr := readBoolAtAddress(client, hb.Address)
	if readErr != nil {
		if ctx.Err() != nil {
			return
		}
		_ = h.varsHandler.UpdateHeartbeatRuntime(context.Background(), hb.ID, hb.LastValue, &now, hb.LastChangeAt, true, readErr.Error())
		return
	}

	lastChangeAt := hb.LastChangeAt
	if hb.LastValue == nil || *hb.LastValue != value {
		t := now
		lastChangeAt = &t
	}
	switch hb.HeartbeatType {
	case db_models.HeartbeatTypeWhenTrueSetFalse:
		if value {
			if err := writeBoolAtAddress(client, hb.Address, false); err != nil {
				if ctx.Err() != nil {
					return
				}
				_ = h.varsHandler.UpdateHeartbeatRuntime(context.Background(), hb.ID, hb.LastValue, &now, hb.LastChangeAt, true, err.Error())
				return
			}
			value = false
			t := now
			lastChangeAt = &t
		}
	case db_models.HeartbeatTypeWhenFalseSetTrue:
		if !value {
			if err := writeBoolAtAddress(client, hb.Address, true); err != nil {
				if ctx.Err() != nil {
					return
				}
				_ = h.varsHandler.UpdateHeartbeatRuntime(context.Background(), hb.ID, hb.LastValue, &now, hb.LastChangeAt, true, err.Error())
				return
			}
			value = true
			t := now
			lastChangeAt = &t
		}
	}
	isFailing := false
	if lastChangeAt != nil && now.Sub(*lastChangeAt) > time.Duration(hb.AlarmSeconds)*time.Second {
		isFailing = true
	}
	v := value
	hb.LastValue = &v
	hb.LastCheckAt = &now
	hb.LastChangeAt = lastChangeAt
	hb.IsFailing = isFailing
	if ctx.Err() != nil {
		return
	}
	_ = h.varsHandler.UpdateHeartbeatRuntime(context.Background(), hb.ID, &v, &now, lastChangeAt, isFailing, "")
}

func normalizeVariables(items []string) []string {
	ret := make([]string, 0, len(items))
	for _, i := range items {
		v := strings.ToUpper(strings.TrimSpace(i))
		if v != "" {
			ret = append(ret, v)
		}
	}
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
	buf := make([]byte, 255)
	v, err := client.Read(address, buf)
	if err != nil {
		return false, err
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Bool:
		return v.(bool), nil
	case reflect.Uint8:
		return v.(uint8) != 0, nil
	case reflect.Int8:
		return v.(int8) != 0, nil
	case reflect.Uint16:
		return v.(uint16) != 0, nil
	case reflect.Int16:
		return v.(int16) != 0, nil
	case reflect.Uint32:
		return v.(uint32) != 0, nil
	case reflect.Int32:
		return v.(int32) != 0, nil
	case reflect.Float32:
		return v.(float32) != 0, nil
	case reflect.Float64:
		return v.(float64) != 0, nil
	default:
		return false, fmt.Errorf("unsupported type for BOOL read: %v", reflect.TypeOf(v).Kind())
	}
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
	return &boolAddressMeta{
		dbNumber:   dbNumber,
		byteOffset: byteOffset,
		bitOffset:  bitOffset,
	}, nil
}
