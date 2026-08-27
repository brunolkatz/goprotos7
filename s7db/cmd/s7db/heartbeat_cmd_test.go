package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHeartbeatDryRunDoesNotRequirePLC(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := filepath.Join(tmp, "s7db.yml")
	content := []byte(`
version: 1
db: 300
endian: big
size: auto
tags:
  - addr: DB300.DBX0.0
    name: OsServiceHeartbeat
    type: BOOL
    role: heartbeat
`)
	if err := os.WriteFile(schemaPath, content, 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	app := &App{
		CLI: &CLI{
			File:   schemaPath,
			DryRun: true,
		},
		Now:     time.Now,
		Timeout: 2 * time.Second,
	}
	cmd := &HeartbeatCmd{
		Count: 1,
	}
	if err := cmd.Run(app); err != nil {
		t.Fatalf("dry-run heartbeat should succeed without plc addr: %v", err)
	}
}
