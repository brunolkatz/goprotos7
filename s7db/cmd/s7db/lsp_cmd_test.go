package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeLSPTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func captureOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()
	oldOut := os.Stdout
	oldErr := os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = outW
	os.Stderr = errW
	runErr := fn()
	_ = outW.Close()
	_ = errW.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)
	_ = outR.Close()
	_ = errR.Close()
	return string(outBytes), string(errBytes), runErr
}

func TestLSPCheckValidScript(t *testing.T) {
	tmp := t.TempDir()
	simPath := writeLSPTestFile(t, tmp, "ok.sim", "tick 100ms;\n")
	app := &App{
		CLI:     &CLI{File: filepath.Join(tmp, "unused.yml")},
		Now:     time.Now,
		Timeout: time.Second,
	}
	cmd := &LSPCmd{Check: simPath, NoSchema: true}
	stdout, _, err := captureOutput(t, func() error { return cmd.Run(app) })
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(stdout, "ok: no diagnostics") {
		t.Fatalf("expected clean output, got %q", stdout)
	}
}

func TestLSPCheckUnterminatedStringReturnsUsageError(t *testing.T) {
	tmp := t.TempDir()
	src := "tick 100ms;\nvar AlarmMessage : STRING[20] := \"\";\nAlarmMessage := \"Meu alarme\n"
	simPath := writeLSPTestFile(t, tmp, "bad.sim", src)
	app := &App{
		CLI:     &CLI{File: filepath.Join(tmp, "unused.yml")},
		Now:     time.Now,
		Timeout: time.Second,
	}
	cmd := &LSPCmd{Check: simPath, NoSchema: true}
	_, stderr, err := captureOutput(t, func() error { return cmd.Run(app) })
	if err == nil {
		t.Fatalf("expected usage error")
	}
	if _, ok := err.(*UsageError); !ok {
		t.Fatalf("expected usage error type, got %T", err)
	}
	if !strings.Contains(stderr, "unterminated string") {
		t.Fatalf("missing diagnostic message: %q", stderr)
	}
	if !strings.Contains(stderr, "range: L2:16-L2:17 (0-based)") {
		t.Fatalf("missing range line: %q", stderr)
	}
	if strings.Contains(stderr, ":1:1:") {
		t.Fatalf("unexpected 1:1 position in output: %q", stderr)
	}
}

func TestLSPCheckIsExclusiveWithStdio(t *testing.T) {
	tmp := t.TempDir()
	simPath := writeLSPTestFile(t, tmp, "f.sim", "tick 100ms;\n")
	app := &App{
		CLI:     &CLI{File: filepath.Join(tmp, "unused.yml")},
		Now:     time.Now,
		Timeout: time.Second,
	}
	cmd := &LSPCmd{Check: simPath, Stdio: true, NoSchema: true}
	err := cmd.Run(app)
	if err == nil {
		t.Fatalf("expected usage error")
	}
	if !strings.Contains(err.Error(), "--check is mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLSPCheckUsesSchemaFile(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := writeLSPTestFile(t, tmp, "s7db.yml", `version: 1
db: 300
endian: big
size: auto
tags:
  - addr: DB300.DBX0.0
    name: MyVar
    type: BOOL
`)
	simPath := writeLSPTestFile(t, tmp, "with-schema.sim", "tick 100ms;\nMyVar := true;\n")
	app := &App{
		CLI:     &CLI{File: schemaPath},
		Now:     time.Now,
		Timeout: time.Second,
	}
	cmd := &LSPCmd{Check: simPath}
	_, _, err := captureOutput(t, func() error { return cmd.Run(app) })
	if err != nil {
		t.Fatalf("expected schema-backed check success, got %v", err)
	}
}

func TestLSPCheckOncePulseParseErrorPosition(t *testing.T) {
	tmp := t.TempDir()
	src := "tick 100ms;\nonce AlarmMessage := \"x\";\n"
	simPath := writeLSPTestFile(t, tmp, "once-bad.sim", src)
	app := &App{
		CLI:     &CLI{File: filepath.Join(tmp, "unused.yml")},
		Now:     time.Now,
		Timeout: time.Second,
	}
	cmd := &LSPCmd{Check: simPath, NoSchema: true}
	_, stderr, err := captureOutput(t, func() error { return cmd.Run(app) })
	if err == nil {
		t.Fatalf("expected usage error")
	}
	if !strings.Contains(stderr, `"once" can only prefix pulse`) {
		t.Fatalf("missing once parse error: %q", stderr)
	}
	if !strings.Contains(stderr, ":2:1: error:") {
		t.Fatalf("expected line/col on once statement, got %q", stderr)
	}
}
