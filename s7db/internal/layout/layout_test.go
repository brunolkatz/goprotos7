package layout_test

import (
	"encoding/binary"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/layout"
	"github.com/brunolkatz/goprotos7/s7db/internal/pack"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

func TestOverlapDetection(t *testing.T) {
	s := schema.Schema{
		Version: 1,
		DB:      10,
		Endian:  "big",
		Size:    schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Addr: "DB10.DBW2", Type: "INT"},
			{Addr: "DB10.DBD2", Type: "DINT"},
		},
	}
	_, err := layout.Build(s, true)
	if err == nil {
		t.Fatalf("expected overlap error")
	}
}

func TestBoolPacking(t *testing.T) {
	s := schema.Schema{
		Version: 1,
		DB:      10,
		Endian:  "big",
		Size:    schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Addr: "DB10.DBX0.0", Type: "BOOL", Init: true},
			{Addr: "DB10.DBX0.7", Type: "BOOL", Init: true},
		},
	}
	got, _, err := pack.Pack(s, 0x00, 0, true)
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}
	if len(got) == 0 || got[0] != 0x81 {
		t.Fatalf("unexpected first byte: %#x", got[0])
	}
}

func TestNumericEndian(t *testing.T) {
	s := schema.Schema{
		Version: 1,
		DB:      10,
		Endian:  "big",
		Size:    schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Addr: "DB10.DBW0", Type: "INT", Init: 258},
			{Addr: "DB10.DBD2", Type: "REAL", Init: 1.5},
		},
	}
	got, _, err := pack.Pack(s, 0x00, 0, true)
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}
	if binary.BigEndian.Uint16(got[0:2]) != 258 {
		t.Fatalf("unexpected int bytes: %#v", got[0:2])
	}
}

func TestStringLayout(t *testing.T) {
	s := schema.Schema{
		Version: 1,
		DB:      10,
		Endian:  "big",
		Size:    schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Addr: "DB10.DBB0", Type: "STRING[4]", Init: "ABCDE"},
		},
	}
	got, _, err := pack.Pack(s, 0x00, 0, true)
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}
	if len(got) != 6 {
		t.Fatalf("unexpected size: %d", len(got))
	}
	if got[0] != 4 || got[1] != 4 {
		t.Fatalf("unexpected string header: %v", got[:2])
	}
}
