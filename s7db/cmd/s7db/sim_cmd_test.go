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
	if err := os.WriteFile(path, []byte("tick 100ms;\n"), 0o644); err != nil {
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

func TestSimWriteAllRequiresWriteAndPlc(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := writeSchemaFixture(t, tmp)
	simPath := writeSimFixture(t, tmp)
	app := &App{
		CLI: &CLI{File: schemaPath},
		Now: time.Now, Timeout: time.Second,
	}
	cmd := &SimCmd{Path: simPath, WriteAll: true}
	if err := cmd.Run(app); err == nil {
		t.Fatalf("expected usage error")
	}
}

func TestSimWriteAllXorAllowWrite(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := writeSchemaFixture(t, tmp)
	simPath := writeSimFixture(t, tmp)
	app := &App{
		CLI: &CLI{File: schemaPath},
		Now: time.Now, Timeout: time.Second,
	}
	cmd := &SimCmd{
		Path: simPath, PLC: true, Write: true,
		WriteAll: true, AllowWrite: "OsServiceHeartbeat",
	}
	if err := cmd.Run(app); err == nil {
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
	out, skipped := filterPushes(changes, allow, s, false, true, false)
	if len(skipped) != 1 || skipped[0] != "OsServiceHeartbeat" {
		t.Fatalf("unexpected skipped heartbeat list: %+v", skipped)
	}
	if len(out) != 1 || out[0].Name != "OsServiceFault" {
		t.Fatalf("unexpected filtered push list: %+v", out)
	}
	out, skipped = filterPushes(changes, allow, s, true, true, false)
	if len(skipped) != 0 {
		t.Fatalf("did not expect skipped heartbeat when take-heartbeat is set")
	}
	if len(out) != 2 {
		t.Fatalf("expected both writes with --take-heartbeat, got %+v", out)
	}
}

func TestFilterPushesWriteAllSkipsHeartbeatByDefault(t *testing.T) {
	s := schema.Schema{
		Tags: []schema.Tag{
			{Name: "OsServiceHeartbeat", Role: "heartbeat"},
			{Name: "OsServiceFault"},
			{Name: "MillSpeed"},
		},
	}
	changes := []sim.Change{
		{Name: "OsServiceHeartbeat", Value: true},
		{Name: "OsServiceFault", Value: true},
		{Name: "MillSpeed", Value: 10.0},
	}
	out, skipped := filterPushes(changes, nil, s, false, true, true)
	if len(out) != 2 {
		t.Fatalf("expected non-heartbeat changes to pass with write-all, got %+v", out)
	}
	if len(skipped) != 1 || skipped[0] != "OsServiceHeartbeat" {
		t.Fatalf("expected heartbeat skip warning target, got %+v", skipped)
	}
}

func TestFilterPushesSkipsTraceEvents(t *testing.T) {
	s := schema.Schema{
		Tags: []schema.Tag{
			{Name: "PopupOKActivationPulse"},
		},
	}
	changes := []sim.Change{
		{Name: "PopupOKActivationPulse", Value: true},
		{Name: "PopupOKActivationPulse", Event: "pulse PopupOKActivationPulse fire n=2 site=x"},
	}
	allow := map[string]struct{}{
		"PopupOKActivationPulse": {},
	}
	out, skipped := filterPushes(changes, allow, s, false, true, false)
	if len(skipped) != 0 {
		t.Fatalf("did not expect skipped heartbeat entries: %+v", skipped)
	}
	if len(out) != 1 {
		t.Fatalf("expected only the value change to be pushed, got %+v", out)
	}
	if out[0].Event != "" {
		t.Fatalf("expected event-only change to be filtered out: %+v", out[0])
	}
}
