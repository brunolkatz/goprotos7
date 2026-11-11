package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

type watchWStringClient struct {
	raw []byte
}

func (c watchWStringClient) ReadDB(db, start, size int) ([]byte, error) {
	if db != 1 || start != 2 || size != len(c.raw) {
		return nil, fmt.Errorf("unexpected read DB%d offset %d size %d", db, start, size)
	}
	return c.raw, nil
}

func (c watchWStringClient) Read(string, int) (any, error) {
	return nil, fmt.Errorf("WSTRING should be decoded from raw DB bytes")
}

func TestWatchWString(t *testing.T) {
	s := schema.Schema{DB: 1, Endian: "big", Tags: []schema.Tag{
		{Name: "Message", Addr: "DB1.WSTRING2.3", Type: "WSTRING"},
	}}
	raw := []byte{0, 3, 0, 2, 0, 0x41, 0, 0xe9, 0, 0}
	for _, name := range []string{"Message", "DB1.WSTRING2.3"} {
		t.Run(name, func(t *testing.T) {
			vars, errs, err := resolveWatchVars(s, []string{name}, nil, "big")
			if err != nil || len(errs) != 0 {
				t.Fatalf("resolve watch vars: %v %v", err, errs)
			}
			app := &App{Now: time.Now}
			out, _, err := captureOutput(t, func() error {
				return (&WatchCmd{Output: "json"}).printSample(context.Background(), app, watchWStringClient{raw}, vars, map[string]watchDiffState{})
			})
			if err != nil || !strings.Contains(out, "Aé") {
				t.Fatalf("watch output %q: %v", out, err)
			}
		})
	}
}

func TestWStringInitAddCheckPackUnpack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.yml")
	db := 1
	app := &App{CLI: &CLI{File: path, DB: &db, Endian: "big", Quiet: true}, Now: time.Now}
	if err := (&InitCmd{InitLikeFlags: InitLikeFlags{Size: "auto"}}).Run(app); err != nil {
		t.Fatal(err)
	}

	if err := (&AddCmd{Type: "WSTRING", Name: "Message", Init: "Aé", Args: []string{"WSTRING2.3"}}).Run(app); err != nil {
		t.Fatal(err)
	}
	if _, _, err := captureOutput(t, func() error { return (&CheckCmd{Output: "json"}).Run(app) }); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "db.bin")
	if err := (&PackCmd{Format: "raw", Fill: "0x00", Output: bin}).Run(app); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 0, 3, 0, 2, 0, 'A', 0, 0xe9, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("packed bytes = % x, want % x", got, want)
	}
	unpacked := filepath.Join(dir, "unpacked.yml")
	if err := (&UnpackCmd{From: path, Input: bin, Output: unpacked}).Run(app); err != nil {
		t.Fatal(err)
	}
	result, err := schema.Load(unpacked, &db)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tags) != 1 || result.Tags[0].Init != "Aé" {
		t.Fatalf("unexpected unpacked tags: %+v", result.Tags)
	}
}

func TestInitImportsWStringSchema(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.yml")
	if err := os.WriteFile(src, []byte("version: 1\ndb: 1\nendian: big\nsize: auto\ntags:\n  - addr: DB1.WSTRING0.4\n    name: Wide\n    type: WSTRING\n    init: café\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "schema.yml")
	app := &App{CLI: &CLI{File: dst, Endian: "big"}, Now: time.Now}
	if err := (&InitCmd{InitLikeFlags: InitLikeFlags{From: src, Size: "auto"}}).Run(app); err != nil {
		t.Fatal(err)
	}
	s, err := schema.Load(dst, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tags) != 1 || s.Tags[0].Addr != "DB1.WSTRING0.4" || s.Tags[0].Init != "café" {
		t.Fatalf("unexpected imported WSTRING: %+v", s.Tags)
	}
}
