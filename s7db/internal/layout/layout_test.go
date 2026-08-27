package layout_test

import (
	"encoding/binary"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
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
			{Addr: "DB10.DBB0", Type: "STRING[4]", Init: "ABCD"},
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

func TestStringTypeLayoutFromAddress(t *testing.T) {
	s := schema.Schema{
		Version: 1,
		DB:      10,
		Endian:  "big",
		Size:    schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Addr: "DB10.STRING10.4", Type: "STRING", Init: "AB"},
		},
	}
	got, _, err := pack.Pack(s, 0x00, 0, true)
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	addrValue, err := address.Parse("DB10.STRING10.4", nil)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if addrValue.StartBit() != 80 {
		t.Fatalf("unexpected start bit: %d", addrValue.StartBit())
	}

	ts, err := layout.ParseType(s.Tags[0])
	if err != nil {
		t.Fatalf("parse type failed: %v", err)
	}
	if ts.StringLen != 4 || ts.SizeBytes != 6 || ts.BitSize != 48 {
		t.Fatalf("unexpected type spec: %+v", ts)
	}

	if len(got) != 16 {
		t.Fatalf("unexpected size: %d", len(got))
	}
	if got[10] != 4 || got[11] != 2 {
		t.Fatalf("unexpected string header at offset 10: %v", got[10:12])
	}
	if string(got[12:14]) != "AB" {
		t.Fatalf("unexpected payload: %q", string(got[12:14]))
	}
}
