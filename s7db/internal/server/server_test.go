package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

type mockPLC struct {
	mu  sync.Mutex
	dbs map[int][]byte
}

func newMockPLC() *mockPLC {
	return &mockPLC{dbs: map[int][]byte{}}
}

func (m *mockPLC) Connect(context.Context) error { return nil }
func (m *mockPLC) Close() error                  { return nil }

func (m *mockPLC) ReadDB(db, start, size int) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	area := m.dbs[db]
	end := start + size
	if len(area) < end {
		expanded := make([]byte, end)
		copy(expanded, area)
		area = expanded
		m.dbs[db] = area
	}
	out := make([]byte, size)
	copy(out, area[start:end])
	return out, nil
}

func (m *mockPLC) WriteDB(db, start int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	area := m.dbs[db]
	end := start + len(data)
	if len(area) < end {
		expanded := make([]byte, end)
		copy(expanded, area)
		area = expanded
		m.dbs[db] = area
	}
	copy(area[start:end], data)
	return nil
}

func testSchema(t *testing.T) schema.Schema {
	t.Helper()
	min := 0.0
	max := 100.0
	s := schema.Schema{
		Version: 1,
		DB:      1,
		Endian:  "big",
		Size:    schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{
				Addr: "DB1.DBX0.0",
				Name: "HB",
				Type: "BOOL",
				Role: "heartbeat",
				Heartbeat: &schema.HeartbeatSpec{
					Interval: "1s",
					Timeout:  "5s",
					Polarity: "set-true",
				},
			},
			{Addr: "DB1.DBX0.1", Name: "Valve", Type: "BOOL"},
			{Addr: "DB1.DBW2", Name: "Setpoint", Type: "INT", UI: &schema.TagUI{Min: &min, Max: &max}},
			{Addr: "DB1.STRING10.8", Name: "Message", Type: "STRING"},
		},
	}
	if err := s.Normalize(&s.DB); err != nil {
		t.Fatalf("normalize schema: %v", err)
	}
	return s
}

func newTestServer(t *testing.T) (*Server, *mockPLC) {
	t.Helper()
	mock := newMockPLC()
	srv, err := newServer(Config{
		Listen:   "127.0.0.1:0",
		Addr:     "127.0.0.1",
		Rack:     0,
		Slot:     1,
		Port:     102,
		Interval: 200 * time.Millisecond,
		Now:      time.Now,
	}, testSchema(t), mock)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	return srv, mock
}

func TestBitRMWPreservesSiblingBits(t *testing.T) {
	srv, mock := newTestServer(t)
	mock.dbs[1] = []byte{0x01, 0x00, 0x00, 0x00}
	tag, ok := srv.findTag("Valve")
	if !ok {
		t.Fatalf("missing tag Valve")
	}
	if _, _, _, err := srv.applyWrite(context.Background(), tag, true, ""); err != nil {
		t.Fatalf("applyWrite failed: %v", err)
	}
	got, _ := mock.ReadDB(1, 0, 1)
	if got[0] != 0x03 {
		t.Fatalf("expected bit0 preserved and bit1 set, got %#x", got[0])
	}
}

func TestHeartbeatSetFalsePreservesSiblingBits(t *testing.T) {
	srv, mock := newTestServer(t)
	mock.dbs[1] = []byte{0x07, 0x00, 0x00, 0x00}
	client := &lockedHeartbeatClient{server: srv}
	if err := client.WriteBool(context.Background(), 1, 0, 0, false); err != nil {
		t.Fatalf("WriteBool false failed: %v", err)
	}
	got, _ := mock.ReadDB(1, 0, 1)
	if got[0] != 0x06 {
		t.Fatalf("expected sibling bits untouched while clearing bit0, got %#x", got[0])
	}
}

func TestHeartbeatTagWriteRejected(t *testing.T) {
	srv, _ := newTestServer(t)
	tag, ok := srv.findTag("HB")
	if !ok {
		t.Fatalf("missing heartbeat tag")
	}
	_, _, code, err := srv.applyWrite(context.Background(), tag, true, "")
	if err == nil {
		t.Fatalf("expected heartbeat protection error")
	}
	if code != "heartbeat_protected" {
		t.Fatalf("expected heartbeat_protected, got %q", code)
	}
}

func TestMinMaxReject(t *testing.T) {
	srv, _ := newTestServer(t)
	tag, ok := srv.findTag("Setpoint")
	if !ok {
		t.Fatalf("missing Setpoint tag")
	}
	_, _, code, err := srv.applyWrite(context.Background(), tag, 101, "")
	if err == nil {
		t.Fatalf("expected out-of-range error")
	}
	if code != "out_of_range" {
		t.Fatalf("expected out_of_range, got %q", code)
	}
}

func TestPutTagHandlerWritesMockPLC(t *testing.T) {
	srv, mock := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tags/Setpoint", strings.NewReader(`{"value":55}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handleTagByName(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	raw, _ := mock.ReadDB(1, 2, 2)
	if binary.BigEndian.Uint16(raw) != 55 {
		t.Fatalf("expected written value 55, got %d", binary.BigEndian.Uint16(raw))
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if ok, _ := payload["ok"].(bool); !ok {
		t.Fatalf("expected ok=true response")
	}
}

func TestStringWrite(t *testing.T) {
	srv, mock := newTestServer(t)
	tag, ok := srv.findTag("Message")
	if !ok {
		t.Fatalf("missing Message tag")
	}
	if _, _, _, err := srv.applyWrite(context.Background(), tag, "HELLO", ""); err != nil {
		t.Fatalf("applyWrite string failed: %v", err)
	}
	raw, _ := mock.ReadDB(1, 10, 10)
	if raw[0] != 8 || raw[1] != 5 {
		t.Fatalf("unexpected S7 string header: %v", raw[:2])
	}
	if string(raw[2:7]) != "HELLO" {
		t.Fatalf("unexpected string payload: %q", string(raw[2:7]))
	}
}
