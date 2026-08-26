package heartbeat

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
)

type mockClient struct {
	mu          sync.Mutex
	connects    int
	reads       []bool
	readIndex   int
	writes      []bool
	connectErrs []error
}

func (m *mockClient) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connects++
	if len(m.connectErrs) > 0 {
		err := m.connectErrs[0]
		m.connectErrs = m.connectErrs[1:]
		return err
	}
	return nil
}

func (m *mockClient) Close() error { return nil }

func (m *mockClient) ReadBool(ctx context.Context, db, by, bit int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readIndex >= len(m.reads) {
		return false, nil
	}
	v := m.reads[m.readIndex]
	m.readIndex++
	return v, nil
}

func (m *mockClient) WriteBool(ctx context.Context, db, by, bit int, value bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, value)
	return nil
}

func fastSleep(ctx context.Context, d time.Duration) error { return nil }

func TestRunSetTrueSequence(t *testing.T) {
	addr, _ := address.Parse("DB300.DBX0.0", nil)
	m := &mockClient{reads: []bool{false, false}}
	err := Run(context.Background(), m, Config{
		Address:     addr,
		Mode:        "set-true",
		Count:       2,
		Interval:    time.Millisecond,
		Timeout:     5 * time.Second,
		ClearWithin: time.Millisecond,
		Sleep:       fastSleep,
		Now:         time.Now,
		Out:         &bytes.Buffer{},
		Err:         &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(m.writes) != 2 || !m.writes[0] || !m.writes[1] {
		t.Fatalf("expected two true writes, got %+v", m.writes)
	}
}

func TestRunToggleSequence(t *testing.T) {
	addr, _ := address.Parse("DB300.DBX0.0", nil)
	m := &mockClient{reads: []bool{false, true}}
	err := Run(context.Background(), m, Config{
		Address:  addr,
		Mode:     "toggle",
		Count:    2,
		Interval: time.Millisecond,
		Timeout:  5 * time.Second,
		Sleep:    fastSleep,
		Now:      time.Now,
		Out:      &bytes.Buffer{},
		Err:      &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(m.writes) != 2 || m.writes[0] != true || m.writes[1] != false {
		t.Fatalf("unexpected toggle writes: %+v", m.writes)
	}
}

func TestRequireClearStrictFailure(t *testing.T) {
	addr, _ := address.Parse("DB300.DBX0.0", nil)
	m := &mockClient{reads: []bool{true, true}}
	err := Run(context.Background(), m, Config{
		Address:      addr,
		Mode:         "set-true",
		Count:        1,
		Interval:     time.Millisecond,
		Timeout:      5 * time.Second,
		RequireClear: true,
		ClearWithin:  time.Millisecond,
		Strict:       true,
		Sleep:        fastSleep,
		Now:          time.Now,
		Out:          &bytes.Buffer{},
		Err:          &bytes.Buffer{},
	})
	if err == nil {
		t.Fatalf("expected strict clear failure")
	}
}

func TestContextCancelStopsLoop(t *testing.T) {
	addr, _ := address.Parse("DB300.DBX0.0", nil)
	m := &mockClient{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, m, Config{
		Address:  addr,
		Mode:     "set-true",
		Interval: time.Millisecond,
		Timeout:  5 * time.Second,
		Sleep:    fastSleep,
		Now:      time.Now,
		Out:      &bytes.Buffer{},
		Err:      &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("expected clean stop on cancel, got: %v", err)
	}
}

func TestReconnectTimeoutFailure(t *testing.T) {
	addr, _ := address.Parse("DB300.DBX0.0", nil)
	m := &mockClient{connectErrs: []error{errors.New("down"), errors.New("down"), errors.New("down")}}
	now := time.Now()
	err := Run(context.Background(), m, Config{
		Address:   addr,
		Mode:      "set-true",
		Count:     1,
		Interval:  time.Millisecond,
		Timeout:   1500 * time.Millisecond,
		Reconnect: true,
		Sleep: func(ctx context.Context, d time.Duration) error {
			now = now.Add(time.Second)
			return nil
		},
		Now: func() time.Time { return now },
		Out: &bytes.Buffer{},
		Err: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatalf("expected reconnect timeout failure")
	}
}
