package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/decode"
	"github.com/brunolkatz/goprotos7/s7db/internal/heartbeat"
	"github.com/brunolkatz/goprotos7/s7db/internal/hmi"
	"github.com/brunolkatz/goprotos7/s7db/internal/plc"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

type PLCClient interface {
	Connect(ctx context.Context) error
	Close() error
	ReadDB(db, start, size int) ([]byte, error)
	WriteDB(db, start int, data []byte) error
}

type Config struct {
	Listen    string
	Token     string
	ReadOnly  bool
	Interval  time.Duration
	Addr      string
	Rack      int
	Slot      int
	Port      int
	Heartbeat bool
	NoOpen    bool
	Force     bool
	Timeout   time.Duration
	Now       func() time.Time
	Out       io.Writer
	Err       io.Writer
	Client    PLCClient
	Verbose   bool
}

type resolvedTag struct {
	Key       string
	Name      string
	Addr      address.Address
	Type      string
	Spec      decode.TypeSpec
	ByteOrder binary.ByteOrder
	UI        schema.TagUI
	Role      string
	Heartbeat *schema.HeartbeatSpec
}

type tagState struct {
	Raw     []byte
	RawHex  string
	RawText string
	Value   any
	Err     string
}

type heartbeatState struct {
	Running  bool   `json:"running"`
	LastBeat string `json:"last_beat,omitempty"`
	Fault    bool   `json:"fault"`
	Error    string `json:"error,omitempty"`
}

type Server struct {
	cfg        Config
	schema     schema.Schema
	client     PLCClient
	order      binary.ByteOrder
	tags       []*resolvedTag
	tagsByName map[string]*resolvedTag

	ioMu    sync.Mutex
	stateMu sync.RWMutex
	state   map[string]tagState

	ws          *wsHub
	setReqCh    chan wsSetRequest
	hb          *heartbeatController
	lastPLCUpMu sync.RWMutex
	lastPLCUp   bool
}

func Run(ctx context.Context, s schema.Schema, cfg Config) error {
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8080"
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 200 * time.Millisecond
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Err == nil {
		cfg.Err = io.Discard
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if !cfg.ReadOnly && cfg.Token == "" && !isLoopbackListen(cfg.Listen) && !cfg.Force {
		return fmt.Errorf("serve: token required for non-loopback writes (use --token, --read-only, or --force)")
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		return fmt.Errorf("serve: --addr is required")
	}

	client := cfg.Client
	if client == nil {
		client = plc.New(cfg.Addr, cfg.Rack, cfg.Slot, cfg.Port, cfg.Timeout)
	}

	srv, err := newServer(cfg, s, client)
	if err != nil {
		return err
	}
	connectCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	if err := srv.client.Connect(connectCtx); err != nil {
		cancel()
		return plcInitError("plc_connect_failed", "unable to connect to PLC", cfg, err)
	}
	cancel()
	if err := srv.verifyStartupConnection(ctx); err != nil {
		_ = srv.client.Close()
		return err
	}
	defer srv.client.Close()
	srv.setPLCUp(true)

	mux, err := srv.router()
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:    cfg.Listen,
		Handler: mux,
	}
	if !cfg.NoOpen {
		fmt.Fprintf(cfg.Out, "http://%s\n", cfg.Listen)
	}

	go srv.pollLoop(ctx)
	go srv.coalesceWSWrites(ctx)
	if cfg.Heartbeat {
		if err := srv.hb.Start(ctx); err != nil {
			return err
		}
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}
	shutCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		return err
	}
	srv.hb.Stop()
	return nil
}

func (s *Server) verifyStartupConnection(ctx context.Context) error {
	if len(s.tags) == 0 {
		return nil
	}
	probeCount := 2
	probeTag := s.tags[0]
	for i := 0; i < probeCount; i++ {
		raw, err := s.readTagRaw(ctx, probeTag)
		if err != nil {
			return plcInitError("plc_unstable_connection", "connected but could not sustain PLC reads during startup", s.cfg, err)
		}
		if len(raw) != probeTag.Spec.SizeBytes {
			return plcInitError(
				"plc_unstable_connection",
				fmt.Sprintf("connected but read size mismatch during startup (got %d expected %d)", len(raw), probeTag.Spec.SizeBytes),
				s.cfg,
				nil,
			)
		}
		if i < probeCount-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	return nil
}

func plcInitError(code, message string, cfg Config, err error) error {
	if err != nil {
		return fmt.Errorf(
			"serve: %s (code=%s addr=%s rack=%d slot=%d port=%d err=%v)",
			message,
			code,
			cfg.Addr,
			cfg.Rack,
			cfg.Slot,
			cfg.Port,
			err,
		)
	}
	return fmt.Errorf(
		"serve: %s (code=%s addr=%s rack=%d slot=%d port=%d)",
		message,
		code,
		cfg.Addr,
		cfg.Rack,
		cfg.Slot,
		cfg.Port,
	)
}

func newServer(cfg Config, s schema.Schema, client PLCClient) (*Server, error) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Err == nil {
		cfg.Err = io.Discard
	}
	order, err := decode.ByteOrder(s.Endian)
	if err != nil {
		return nil, fmt.Errorf("serve: invalid schema endian %q", s.Endian)
	}
	db := s.DB
	tags := make([]*resolvedTag, 0, len(s.Tags))
	tagsByName := make(map[string]*resolvedTag, len(s.Tags))
	for _, tag := range s.Tags {
		addrValue, err := address.Parse(tag.Addr, &db)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		spec, err := decode.ResolveSchemaType(tag.Type, addrValue)
		if err != nil {
			return nil, fmt.Errorf("serve: %s: %w", tag.Addr, err)
		}
		key := strings.TrimSpace(tag.Name)
		if key == "" {
			key = addrValue.Canonical()
		}
		res := &resolvedTag{
			Key:       key,
			Name:      key,
			Addr:      addrValue,
			Type:      strings.ToUpper(strings.TrimSpace(tag.Type)),
			Spec:      spec,
			ByteOrder: order,
			UI:        hmi.ResolveUI(tag),
			Role:      strings.ToLower(strings.TrimSpace(tag.Role)),
			Heartbeat: tag.Heartbeat,
		}
		tags = append(tags, res)
		tagsByName[strings.ToLower(key)] = res
	}

	out := &Server{
		cfg:        cfg,
		schema:     s,
		client:     client,
		order:      order,
		tags:       tags,
		tagsByName: tagsByName,
		state:      map[string]tagState{},
		ws:         newWSHub(),
		setReqCh:   make(chan wsSetRequest, 256),
		lastPLCUp:  true,
	}
	out.hb = newHeartbeatController(out)
	return out, nil
}

func (s *Server) router() (http.Handler, error) {
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", withNoCache(http.FileServer(http.FS(staticSub))))
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/schema", s.handleSchema)
	mux.HandleFunc("/api/v1/tags", s.handleTags)
	mux.HandleFunc("/api/v1/tags/", s.handleTagByName)
	mux.HandleFunc("/api/v1/heartbeat", s.handleHeartbeatStatus)
	mux.HandleFunc("/api/v1/heartbeat/start", s.handleHeartbeatStart)
	mux.HandleFunc("/api/v1/heartbeat/stop", s.handleHeartbeatStop)
	return mux, nil
}

func (s *Server) pollLoop(ctx context.Context) {
	tick := time.NewTicker(s.cfg.Interval)
	defer tick.Stop()
	for {
		s.pollOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Server) pollOnce(ctx context.Context) {
	updates := make([]map[string]any, 0)
	hadReadError := false
	hadReadSuccess := false
	for _, t := range s.tags {
		raw, err := s.readTagRaw(ctx, t)
		if err != nil {
			hadReadError = true
			continue
		}
		hadReadSuccess = true
		val, decErr := decode.DecodeRaw(t.Spec, t.ByteOrder, raw, t.Addr.Bit)
		state := tagState{
			Raw:     append([]byte(nil), raw...),
			RawHex:  compactHex(raw),
			RawText: spacedHex(raw),
			Value:   val,
		}
		if decErr != nil {
			state.Err = decErr.Error()
			state.Value = "err"
		}

		s.stateMu.Lock()
		prev, exists := s.state[t.Key]
		changed := !exists || !bytes.Equal(prev.Raw, state.Raw) || prev.Err != state.Err
		s.state[t.Key] = state
		s.stateMu.Unlock()
		if changed {
			updates = append(updates, map[string]any{
				"op":    "upd",
				"ts":    s.cfg.Now().UTC().Format(time.RFC3339Nano),
				"name":  t.Key,
				"raw":   state.RawText,
				"value": state.Value,
			})
		}
	}
	if hadReadSuccess {
		s.setPLCUp(true)
	} else if hadReadError {
		s.setPLCUp(false)
	}
	for _, payload := range updates {
		s.ws.broadcastByTag(payload, payload["name"].(string))
	}
	s.ws.broadcast(map[string]any{
		"op":        "hb",
		"running":   s.hb.Status().Running,
		"fault":     s.hb.Status().Fault,
		"last_beat": s.hb.Status().LastBeat,
		"readback":  false,
	})
}

func (s *Server) readTagRaw(ctx context.Context, tag *resolvedTag) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	return s.client.ReadDB(tag.Addr.DB, tag.Addr.Byte, tag.Spec.SizeBytes)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	status := s.hb.Status()
	writeJSON(w, http.StatusOK, map[string]any{
		"plc":       s.isPLCUp(),
		"listen":    s.cfg.Listen,
		"readonly":  s.cfg.ReadOnly,
		"heartbeat": status,
	})
}

func (s *Server) handleSchema(w http.ResponseWriter, _ *http.Request) {
	tags := make([]map[string]any, 0, len(s.tags))
	for _, t := range s.tags {
		tags = append(tags, map[string]any{
			"name": t.Key,
			"addr": t.Addr.Canonical(),
			"type": t.Type,
			"role": t.Role,
			"ui":   t.UI,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema": s.schema,
		"tags":   tags,
	})
}

func (s *Server) handleTags(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"tags": s.snapshotTags(nil),
	})
}

func (s *Server) handleTagByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(path.Base(r.URL.Path))
	if name == "" || name == "tags" {
		http.NotFound(w, r)
		return
	}
	tag, ok := s.findTag(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "tag_not_found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.snapshotTag(tag))
		return
	case http.MethodPut:
		if !s.isWriteAllowed(r, w) {
			return
		}
		var req struct {
			Value any    `json:"value"`
			Raw   string `json:"raw"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
			return
		}
		raw, value, errCode, err := s.applyWrite(r.Context(), tag, req.Value, req.Raw)
		if err != nil {
			status := http.StatusBadRequest
			if errCode == "heartbeat_protected" {
				status = http.StatusForbidden
			}
			writeJSON(w, status, map[string]any{"error": errCode, "detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    true,
			"name":  tag.Key,
			"raw":   spacedHex(raw),
			"value": value,
		})
		return
	default:
		w.Header().Set("Allow", "GET, PUT")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleHeartbeatStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.hb.Status())
}

func (s *Server) handleHeartbeatStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.isWriteAllowed(r, w) {
		return
	}
	if err := s.hb.Start(r.Context()); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "heartbeat_start_failed", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleHeartbeatStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.isWriteAllowed(r, w) {
		return
	}
	s.hb.Stop()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) isWriteAllowed(r *http.Request, w http.ResponseWriter) bool {
	if s.cfg.ReadOnly {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "read_only"})
		return false
	}
	if !s.hasValidToken(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return false
	}
	return true
}

func (s *Server) hasValidToken(r *http.Request) bool {
	if s.cfg.Token == "" {
		return true
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[len("Bearer "):])
		}
	}
	return token == s.cfg.Token
}

func (s *Server) snapshotTags(filter map[string]struct{}) []map[string]any {
	out := make([]map[string]any, 0, len(s.tags))
	for _, t := range s.tags {
		if filter != nil {
			if _, ok := filter[strings.ToLower(t.Key)]; !ok {
				continue
			}
		}
		out = append(out, s.snapshotTag(t))
	}
	return out
}

func (s *Server) snapshotTag(t *resolvedTag) map[string]any {
	s.stateMu.RLock()
	st, ok := s.state[t.Key]
	s.stateMu.RUnlock()
	if !ok {
		st = tagState{Raw: []byte{}, RawHex: "", RawText: "", Value: nil}
	}
	return map[string]any{
		"name":     t.Key,
		"addr":     t.Addr.Canonical(),
		"type":     t.Type,
		"role":     t.Role,
		"ui":       t.UI,
		"raw":      st.RawText,
		"raw_hex":  st.RawHex,
		"raw_text": st.RawText,
		"value":    st.Value,
	}
}

func (s *Server) findTag(name string) (*resolvedTag, bool) {
	tag, ok := s.tagsByName[strings.ToLower(strings.TrimSpace(name))]
	return tag, ok
}

func (s *Server) applyWrite(ctx context.Context, tag *resolvedTag, value any, rawInput string) ([]byte, any, string, error) {
	if tag.Role == "heartbeat" {
		return nil, nil, "heartbeat_protected", fmt.Errorf("tag %s is heartbeat protected", tag.Key)
	}
	if strings.TrimSpace(rawInput) == "" && (tag.UI.Min != nil || tag.UI.Max != nil) {
		v, ok := asFloat(value)
		if !ok {
			return nil, nil, "invalid_value", fmt.Errorf("tag %s requires numeric value", tag.Key)
		}
		if tag.UI.Min != nil && v < *tag.UI.Min {
			return nil, nil, "out_of_range", fmt.Errorf("value %v is below min %v", v, *tag.UI.Min)
		}
		if tag.UI.Max != nil && v > *tag.UI.Max {
			return nil, nil, "out_of_range", fmt.Errorf("value %v is above max %v", v, *tag.UI.Max)
		}
	}

	var payload []byte
	var err error
	if strings.TrimSpace(rawInput) != "" {
		payload, err = parseRawHex(rawInput)
		if err != nil {
			return nil, nil, "invalid_raw", err
		}
		if len(payload) != tag.Spec.SizeBytes {
			return nil, nil, "invalid_raw_size", fmt.Errorf("raw size %d does not match %d", len(payload), tag.Spec.SizeBytes)
		}
	} else {
		payload, err = decode.EncodeRaw(tag.Spec, tag.ByteOrder, value)
		if err != nil {
			return nil, nil, "invalid_value", err
		}
	}

	before, after, err := s.writeTagPayload(ctx, tag, payload)
	if err != nil {
		return nil, nil, "write_failed", err
	}
	decoded, decErr := decode.DecodeRaw(tag.Spec, tag.ByteOrder, after, tag.Addr.Bit)
	if decErr != nil {
		decoded = "err"
	}
	fmt.Fprintf(s.cfg.Err, "s7db: serve: write name=%s addr=%s raw:%s->%s value=%v\n", tag.Key, tag.Addr.Canonical(), compactHex(before), compactHex(after), decoded)

	s.stateMu.Lock()
	s.state[tag.Key] = tagState{
		Raw:     append([]byte(nil), after...),
		RawHex:  compactHex(after),
		RawText: spacedHex(after),
		Value:   decoded,
	}
	s.stateMu.Unlock()
	s.ws.broadcastByTag(map[string]any{
		"op":    "upd",
		"ts":    s.cfg.Now().UTC().Format(time.RFC3339Nano),
		"name":  tag.Key,
		"raw":   spacedHex(after),
		"value": decoded,
	}, tag.Key)
	return after, decoded, "", nil
}

func setDBByteBit(raw []byte, bit int, value bool) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty DB byte")
	}
	if bit < 0 || bit > 7 {
		return fmt.Errorf("invalid bit offset %d", bit)
	}
	mask := byte(1 << bit)
	if value {
		raw[0] |= mask
	} else {
		raw[0] &^= mask
	}
	return nil
}

func (s *Server) writeTagPayload(ctx context.Context, tag *resolvedTag, payload []byte) ([]byte, []byte, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if tag.Spec.Name == "BOOL" {
		current, err := s.client.ReadDB(tag.Addr.DB, tag.Addr.Byte, 1)
		if err != nil {
			return nil, nil, err
		}
		before := append([]byte(nil), current...)
		if len(payload) == 0 {
			payload = []byte{0}
		}
		if err := setDBByteBit(current, tag.Addr.Bit, len(payload) > 0 && payload[0] != 0); err != nil {
			return nil, nil, err
		}
		if err := s.client.WriteDB(tag.Addr.DB, tag.Addr.Byte, current); err != nil {
			return nil, nil, err
		}
		return before, current, nil
	}
	before, err := s.client.ReadDB(tag.Addr.DB, tag.Addr.Byte, len(payload))
	if err != nil {
		return nil, nil, err
	}
	if err := s.client.WriteDB(tag.Addr.DB, tag.Addr.Byte, payload); err != nil {
		return nil, nil, err
	}
	return before, payload, nil
}

func (s *Server) setPLCUp(v bool) {
	s.lastPLCUpMu.Lock()
	s.lastPLCUp = v
	s.lastPLCUpMu.Unlock()
}

func (s *Server) isPLCUp() bool {
	s.lastPLCUpMu.RLock()
	defer s.lastPLCUpMu.RUnlock()
	return s.lastPLCUp
}

type heartbeatController struct {
	server *Server
	mu     sync.Mutex
	cancel context.CancelFunc
	state  heartbeatState
}

func newHeartbeatController(s *Server) *heartbeatController {
	return &heartbeatController{server: s}
}

func (h *heartbeatController) Start(parent context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state.Running {
		return nil
	}
	targets := h.server.findHeartbeatTag()
	if targets == nil {
		return fmt.Errorf("no heartbeat tag found in schema")
	}
	interval := time.Second
	timeout := 5 * time.Second
	ctx, cancel := context.WithCancel(parent)
	h.cancel = cancel
	h.state.Running = true
	h.state.Fault = false
	h.state.Error = ""
	client := &lockedHeartbeatClient{server: h.server, onBeat: h.markBeat}
	if len(targets) > 0 {
		mode := "set-true"
		for _, target := range targets {
			if strings.TrimSpace(target.Heartbeat.Interval) != "" {
				v, err := time.ParseDuration(target.Heartbeat.Interval)
				if err != nil {
					return fmt.Errorf("invalid heartbeat interval: %w", err)
				}
				interval = v
			}
			if strings.TrimSpace(target.Heartbeat.Timeout) != "" {
				v, err := time.ParseDuration(target.Heartbeat.Timeout)
				if err != nil {
					return fmt.Errorf("invalid heartbeat timeout: %w", err)
				}
				timeout = v
			}
			if strings.TrimSpace(target.Heartbeat.Polarity) != "" {
				mode = strings.TrimSpace(target.Heartbeat.Polarity)
			}

			go func() {
				err := heartbeat.Run(ctx, client, heartbeat.Config{ // on serve command
					Verbose:      h.server.cfg.Verbose,
					Address:      target.Addr,
					Name:         target.Key,
					Interval:     interval,
					Timeout:      timeout,
					Mode:         mode,
					Reconnect:    true,
					NoReadback:   true,
					RequireClear: false,
					Quiet:        true,
					Now:          h.server.cfg.Now,
					Out:          io.Discard,
					Err:          h.server.cfg.Err,
				})
				h.mu.Lock()
				defer h.mu.Unlock()
				h.state.Running = false
				if err != nil && !errors.Is(err, context.Canceled) {
					h.state.Fault = true
					h.state.Error = err.Error()
				}
			}()
		}
	}

	return nil
}

func (h *heartbeatController) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.state.Running = false
}

func (h *heartbeatController) markBeat() {
	h.mu.Lock()
	h.state.LastBeat = h.server.cfg.Now().UTC().Format(time.RFC3339Nano)
	h.mu.Unlock()
}

func (h *heartbeatController) Status() heartbeatState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (s *Server) findHeartbeatTag() []*resolvedTag {
	var ret []*resolvedTag
	for i := range s.tags {
		if strings.EqualFold(strings.TrimSpace(s.tags[i].Role), "heartbeat") {
			ret = append(ret, s.tags[i])
		}
	}
	return ret
}

type lockedHeartbeatClient struct {
	server *Server
	onBeat func()
}

func (c *lockedHeartbeatClient) Connect(ctx context.Context) error {
	c.server.ioMu.Lock()
	defer c.server.ioMu.Unlock()
	return c.server.client.Connect(ctx)
}

func (c *lockedHeartbeatClient) Close() error {
	c.server.ioMu.Lock()
	defer c.server.ioMu.Unlock()
	return c.server.client.Close()
}

func (c *lockedHeartbeatClient) ReadBool(ctx context.Context, db, by, bit int) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	c.server.ioMu.Lock()
	defer c.server.ioMu.Unlock()
	buf, err := c.server.client.ReadDB(db, by, 1)
	if err != nil {
		return false, err
	}
	return (buf[0]&(1<<bit) != 0), nil
}

func (c *lockedHeartbeatClient) WriteBool(ctx context.Context, db, by, bit int, value bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	c.server.ioMu.Lock()
	defer c.server.ioMu.Unlock()
	buf, err := c.server.client.ReadDB(db, by, 1)
	if err != nil {
		return err
	}
	if err := setDBByteBit(buf, bit, value); err != nil {
		return err
	}
	if err := c.server.client.WriteDB(db, by, buf); err != nil {
		return err
	}
	if value && c.onBeat != nil {
		c.onBeat()
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func withNoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ".") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func parseRawHex(in string) ([]byte, error) {
	raw := strings.ToUpper(strings.TrimSpace(in))
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "0X", "")
	if raw == "" {
		return nil, fmt.Errorf("raw hex is empty")
	}
	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("raw hex must have even length")
	}
	out, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid raw hex %q", in)
	}
	return out, nil
}

func spacedHex(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	parts := make([]string, 0, len(raw))
	for _, b := range raw {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return strings.Join(parts, " ")
}

func compactHex(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(raw))
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func isLoopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		host = listen
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
