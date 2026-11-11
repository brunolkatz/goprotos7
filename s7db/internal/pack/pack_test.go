package pack_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/pack"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

func TestWStringPackUnpack(t *testing.T) {
	for _, endian := range []string{"big", "little"} {
		t.Run(endian, func(t *testing.T) {
			s := schema.New(10)
			s.Endian = endian
			s.Tags = []schema.Tag{
				{Addr: "DB10.DBB2", Type: "WSTRING[3]", Init: "A\u00e9\u4e2d"},
				{Addr: "DB10.WSTRING12.2", Type: "WSTRING"},
				{Addr: "DB10.DBB20", Type: "STRING[2]", Init: "AB"},
				{Addr: "DB10.DBW24", Type: "WORD", Init: 0x1234},
			}
			raw, computed, err := pack.Pack(s, 0xaa, 8, true)
			if err != nil {
				t.Fatal(err)
			}
			if computed != 26 || len(raw) != 32 {
				t.Fatalf("size: computed=%d padded=%d", computed, len(raw))
			}
			if !bytes.Equal(raw[2:12], []byte{0, 3, 0, 3, 0, 'A', 0, 0xe9, 0x4e, 0x2d}) {
				t.Fatalf("invalid WSTRING bytes: %x", raw[2:12])
			}
			if !bytes.Equal(raw[12:20], []byte{0, 2, 0, 0, 0, 0, 0, 0}) {
				t.Fatalf("invalid empty WSTRING: %x", raw[12:20])
			}
			if !bytes.Equal(raw[20:24], []byte{2, 2, 'A', 'B'}) {
				t.Fatalf("STRING changed: %x", raw[20:24])
			}
			word := []byte{0x12, 0x34}
			if endian == "little" {
				word = []byte{0x34, 0x12}
			}
			if !bytes.Equal(raw[24:26], word) || raw[0] != 0xaa || raw[31] != 0xaa {
				t.Fatalf("numeric endian/fill changed: %x", raw)
			}
			out, err := pack.Unpack(raw, 10, &s)
			if err != nil {
				t.Fatal(err)
			}
			if out.Tags[0].Init != s.Tags[0].Init || out.Tags[1].Init != "" || out.Tags[2].Init != "AB" {
				t.Fatalf("unexpected tags: %+v", out.Tags)
			}
			repacked, _, err := pack.Pack(out, 0xaa, 8, true)
			if err != nil || !bytes.Equal(repacked, raw) {
				t.Fatalf("repack differs: %x, %v", repacked, err)
			}
			if s.Tags[1].Init != nil {
				t.Fatal("unpack modified template")
			}
		})
	}
}

func TestWStringPackRejectsInvalidValues(t *testing.T) {
	s := schema.New(10)
	s.Tags = []schema.Tag{{Addr: "DB10.DBB0", Type: "WSTRING[2]"}}
	for _, value := range []string{"ABC", "\xff", "\U0001f600"} {
		s.Tags[0].Init = value
		if _, _, err := pack.Pack(s, 0, 0, true); err == nil || !strings.Contains(err.Error(), "DB10.DBB0") {
			t.Fatalf("expected contextual error for %q, got %v", value, err)
		}
	}
}

func TestWStringUnpackRejectsMalformedData(t *testing.T) {
	s := schema.New(10)
	s.Tags = []schema.Tag{{Addr: "DB10.DBB0", Type: "WSTRING[2]"}}
	for _, raw := range [][]byte{
		nil,
		{0, 2, 0, 1, 0, 'A'},
		{0, 1, 0, 0, 0, 0, 0, 0},
		{0, 2, 0, 3, 0, 0, 0, 0},
		{0, 2, 0, 1, 0xd8, 0, 0, 0},
	} {
		if _, err := pack.Unpack(raw, 10, &s); err == nil || !strings.Contains(err.Error(), "DB10.DBB0") {
			t.Fatalf("expected contextual error for %x, got %v", raw, err)
		}
	}
}

func TestStringUnpackStillClampsMalformedHeader(t *testing.T) {
	s := schema.New(10)
	s.Tags = []schema.Tag{{Addr: "DB10.DBB0", Type: "STRING[2]"}}
	out, err := pack.Unpack([]byte{255, 255, 'A', 'B'}, 10, &s)
	if err != nil || out.Tags[0].Init != "AB" {
		t.Fatalf("STRING behavior changed: %+v, %v", out, err)
	}
	out, err = pack.Unpack(nil, 10, &s)
	if err != nil || out.Tags[0].Init != nil {
		t.Fatalf("short STRING behavior changed: %+v, %v", out, err)
	}
}
