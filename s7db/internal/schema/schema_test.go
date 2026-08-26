package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHeartbeatNormalizationAndValidation(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "s.yml")
	content := []byte(`
version: 1
db: 300
endian: big
size: auto
tags:
  - addr: DB300.DBX0.0
    name: HB
    type: BOOL
    heartbeat:
      interval: 1s
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write test schema: %v", err)
	}
	s, err := Load(path, nil)
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	if len(s.Tags) != 1 {
		t.Fatalf("expected one tag")
	}
	tag := s.Tags[0]
	if tag.Role != "heartbeat" {
		t.Fatalf("role should normalize to heartbeat, got %q", tag.Role)
	}
	if tag.Heartbeat == nil {
		t.Fatalf("expected heartbeat block")
	}
	if tag.Heartbeat.Interval != "1s" || tag.Heartbeat.Timeout != "5s" || tag.Heartbeat.Polarity != "set-true" {
		t.Fatalf("unexpected heartbeat defaults: %+v", tag.Heartbeat)
	}
	diags, err := Validate(s, true)
	if err != nil {
		t.Fatalf("validate strict failed: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", diags)
	}
}

func TestHeartbeatValidationStrictVsWarning(t *testing.T) {
	s := Schema{
		Version: 1,
		DB:      300,
		Endian:  "big",
		Size:    SizeSpec{Auto: true},
		Tags: []Tag{
			{
				Addr: "DB300.DBW2",
				Type: "INT",
				Role: "heartbeat",
				Heartbeat: &HeartbeatSpec{
					Interval: "5s",
					Timeout:  "5s",
					Polarity: "set-true",
				},
			},
		},
	}
	if err := s.Normalize(&s.DB); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if _, err := Validate(s, true); err == nil {
		t.Fatalf("expected strict validation error")
	}
	diags, err := Validate(s, false)
	if err != nil {
		t.Fatalf("non-strict validate should warn, got err: %v", err)
	}
	if len(diags) < 2 {
		t.Fatalf("expected warnings for non-bool and interval>=timeout, got %+v", diags)
	}
}
