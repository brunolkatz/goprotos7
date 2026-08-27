package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
	"github.com/brunolkatz/goprotos7/s7db/internal/sim"
)

func writeSimFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "f.sim")
	if err := os.WriteFile(path, []byte("tick 100ms\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeSchemaFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "s7db.yml")
	content := []byte(`version: 1
db: 300
endian: big
size: auto
tags:
  - addr: DB300.DBX0.0
    name: OsServiceHeartbeat
    type: BOOL
    role: heartbeat
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSimWriteRequiresAllowWrite(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := writeSchemaFixture(t, tmp)
	simPath := writeSimFixture(t, tmp)
	app := &App{
		CLI: &CLI{
			File: schemaPath,
		},
		Now:     time.Now,
		Timeout: time.Second,
	}
	cmd := &SimCmd{
		Path:  simPath,
		PLC:   true,
		Write: true,
	}
	err := cmd.Run(app)
	if err == nil {
		t.Fatalf("expected usage error")
	}
}

func TestFilterPushesAllowWriteAndHeartbeatProtection(t *testing.T) {
	s := schema.Schema{
		Tags: []schema.Tag{
			{Name: "OsServiceHeartbeat", Role: "heartbeat"},
			{Name: "OsServiceFault"},
		},
	}
	changes := []sim.Change{
		{Name: "OsServiceHeartbeat", Value: true},
		{Name: "OsServiceFault", Value: true},
	}
	allow := map[string]struct{}{
		"OsServiceHeartbeat": {},
		"OsServiceFault":     {},
	}
	out := filterPushes(changes, allow, s, false, true)
	if len(out) != 1 || out[0].Name != "OsServiceFault" {
		t.Fatalf("unexpected filtered push list: %+v", out)
	}
	out = filterPushes(changes, allow, s, true, true)
	if len(out) != 2 {
		t.Fatalf("expected both writes with --take-heartbeat, got %+v", out)
	}
}
