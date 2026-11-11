package layout_test

import (
	"encoding/binary"
	"fmt"
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

func TestAddressOutOfRange(t *testing.T) {
	tests := []struct {
		name  string
		s     schema.Schema
		want  int
		cause string
	}{
		{
			name:  "bit invalid",
			s:     schema.Schema{DB: 10, Size: schema.SizeSpec{Bytes: 128}, Tags: []schema.Tag{{Addr: "DB10.DBX0.8", Name: "ValveOpen", Type: "BOOL"}}},
			want:  1,
			cause: "bit 8 is not 0..7",
		},
		{
			name:  "offset overflow",
			s:     schema.Schema{DB: 10, Size: schema.SizeSpec{Bytes: 128}, Tags: []schema.Tag{{Addr: "DB10.DBW200", Name: "Setpoint", Type: "INT"}}},
			want:  1,
			cause: "offset 200+2 exceeds DB size 128",
		},
		{
			name:  "db mismatch",
			s:     schema.Schema{DB: 10, Size: schema.SizeSpec{Bytes: 128}, Tags: []schema.Tag{{Addr: "DB300.DBD0", Name: "Level", Type: "REAL"}}},
			want:  1,
			cause: "address DB300 but schema db is 10",
		},
		{
			name: "valid",
			s:    schema.Schema{DB: 10, Size: schema.SizeSpec{Bytes: 128}, Tags: []schema.Tag{{Addr: "DB10.DBW0", Name: "Setpoint", Type: "INT"}}},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := layout.AddressOutOfRange(tc.s)
			if len(got) != tc.want {
				t.Fatalf("unexpected findings: got %d want %d", len(got), tc.want)
			}
			if tc.want == 1 && got[0].Cause != tc.cause {
				t.Fatalf("unexpected cause: got %q want %q", got[0].Cause, tc.cause)
			}
		})
	}
}

func TestWStringLayoutTypes(t *testing.T) {
	for _, n := range []int{1, 254, 255, 256, 16382} {
		for _, tag := range []schema.Tag{
			{Addr: "DB10.DBB0", Type: fmt.Sprintf(" wstring[%d] ", n)},
			{Addr: fmt.Sprintf("DB10.WSTRING0.%d", n), Type: "WSTRING"},
		} {
			spec, err := layout.ParseType(tag)
			if err != nil {
				t.Fatal(err)
			}
			if spec.StringLen != n || spec.SizeBytes != 4+2*n || spec.BitSize != (4+2*n)*8 {
				t.Fatalf("unexpected layout for %+v: %+v", tag, spec)
			}
		}
	}
	for _, typ := range []string{"WSTRING[0]", "WSTRING[16383]", "WSTRING[-1]", "WSTRING[]", "WSTRING[1.5]", "WSTRING[99999999999999999999999]"} {
		if _, err := layout.ParseType(schema.Tag{Addr: "DB10.DBB0", Type: typ}); err == nil {
			t.Errorf("accepted invalid type %q", typ)
		}
	}
}

func TestWStringAddressForms(t *testing.T) {
	for _, tc := range []struct {
		addr string
		typ  string
	}{
		{addr: "DB1.WSTRING0.10", typ: "WSTRING"},
		{addr: "WSTRING0.10", typ: "WSTRING"},
		{addr: "DB1.WSTRING0.10", typ: "WSTRING[10]"},
		{addr: "WSTRING0.10", typ: "WSTRING[10]"},
		{addr: "DB1.DBB0", typ: "WSTRING[10]"},
	} {
		t.Run(tc.addr+"/"+tc.typ, func(t *testing.T) {
			s := schema.New(1)
			s.Tags = []schema.Tag{{Addr: tc.addr, Type: tc.typ, Init: "\u4e2d"}}
			if err := s.Normalize(nil); err != nil {
				t.Fatal(err)
			}
			result, err := layout.Build(s, true)
			if err != nil {
				t.Fatal(err)
			}
			tag := result.Tags[0]
			if tag.StartBit != 0 || tag.EndBit != 192 || tag.Type.StringLen != 10 || result.ComputedSize != 24 {
				t.Fatalf("capacity was treated as a bit offset: %+v", result)
			}
			raw, _, err := pack.Pack(s, 0, 0, true)
			if err != nil {
				t.Fatal(err)
			}
			out, err := pack.Unpack(raw, 1, &s)
			if err != nil || out.Tags[0].Init != "\u4e2d" {
				t.Fatalf("address form roundtrip failed: %+v, %v", out, err)
			}
		})
	}
}

func TestBareWStringLayoutRequiresWStringAddress(t *testing.T) {
	for _, raw := range []string{"DB1.DBX0.3", "DB1.DBB0", "DB1.DBW0", "DB1.DBD0", "DB1.STRING0.10"} {
		s := schema.New(1)
		s.Tags = []schema.Tag{{Addr: raw, Type: "WSTRING"}}
		if _, err := layout.ParseType(s.Tags[0]); err == nil {
			t.Errorf("ParseType accepted bare WSTRING at %s", raw)
		}
		for _, strict := range []bool{false, true} {
			if _, err := layout.Build(s, strict); err == nil {
				t.Errorf("Build accepted bare WSTRING at %s (strict=%v)", raw, strict)
			}
		}
	}
}

func TestWStringLayoutRejectsConflictingCapacity(t *testing.T) {
	for _, raw := range []string{"DB1.WSTRING0.20", "WSTRING0.20"} {
		s := schema.New(1)
		s.Tags = []schema.Tag{{Addr: raw, Type: "WSTRING[10]"}}
		if err := s.Normalize(nil); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.ParseType(s.Tags[0]); err == nil {
			t.Fatalf("ParseType accepted conflicting capacity at %s", raw)
		}
		for _, strict := range []bool{false, true} {
			if _, err := layout.Build(s, strict); err == nil {
				t.Fatalf("Build accepted conflicting capacity at %s (strict=%v)", raw, strict)
			}
			if _, _, err := pack.Pack(s, 0, 0, strict); err == nil {
				t.Fatalf("Pack accepted conflicting capacity at %s (strict=%v)", raw, strict)
			}
		}
		if _, err := pack.Unpack(make([]byte, 44), 1, &s); err == nil {
			t.Fatalf("Unpack accepted conflicting capacity at %s", raw)
		}
	}
}

func TestWStringLayoutBoundsAndOverlap(t *testing.T) {
	s := schema.New(10)
	s.Tags = []schema.Tag{
		{Addr: "DB10.WSTRING2.3", Type: "WSTRING"},
		{Addr: "DB10.DBB12", Type: "BYTE"},
	}
	got, err := layout.Build(s, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.ComputedSize != 13 || got.Tags[0].StartBit != 16 || got.Tags[0].EndBit != 96 {
		t.Fatalf("unexpected layout: %+v", got)
	}
	if findings := layout.AddressOutOfRange(s); len(findings) != 0 {
		t.Fatalf("unexpected range findings: %+v", findings)
	}
	s.Tags[1].Addr = "DB10.DBB11"
	if _, err := layout.Build(s, true); err == nil {
		t.Fatal("expected overlap at final WSTRING byte")
	}
	s.Tags = s.Tags[:1]
	s.Size = schema.SizeSpec{Bytes: 11}
	if _, err := layout.Build(s, true); err == nil {
		t.Fatal("expected declared size error")
	}
	findings := layout.AddressOutOfRange(s)
	if len(findings) != 1 || findings[0].Size != 10 || findings[0].Offset != 2 || findings[0].DBSize != 11 {
		t.Fatalf("unexpected range findings: %+v", findings)
	}
	s.Size.Bytes = 12
	if _, err := layout.Build(s, true); err != nil {
		t.Fatalf("exact size failed: %v", err)
	}
	for _, area := range []string{"DBB", "DBW", "DBD"} {
		s.Tags[0] = schema.Tag{Addr: "DB10." + area + "2", Type: "WSTRING[3]"}
		if _, err := layout.Build(s, true); err != nil {
			t.Fatalf("explicit WSTRING at %s: %v", area, err)
		}
	}
	s.Tags[0] = schema.Tag{Addr: "DB10.DBX2.0", Type: "WSTRING[3]"}
	if _, err := layout.Build(s, true); err == nil {
		t.Fatal("accepted WSTRING on BOOL address in strict mode")
	}
}
