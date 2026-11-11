package decode_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/decode"
)

func TestWStringTypeResolution(t *testing.T) {
	for _, n := range []int{1, 254, 255, 256, 16382} {
		for _, typ := range []string{"WSTRING", fmt.Sprintf(" wstring[%d] ", n)} {
			spec, err := decode.ResolveSchemaType(typ, address.Address{Area: "WSTRING", Bit: n})
			if err != nil {
				t.Fatal(err)
			}
			if spec.Name != "WSTRING" || spec.StringLen != n || spec.SizeBytes != 4+2*n {
				t.Fatalf("unexpected spec: %+v", spec)
			}
		}
	}
	for _, typ := range []string{"WSTRING[0]", "WSTRING[16383]", "WSTRING[-1]", "WSTRING[]", "WSTRING[1.5]", "WSTRING[999999999999999999999999999]"} {
		if _, err := decode.ResolveSchemaType(typ, address.Address{}); err == nil {
			t.Errorf("accepted %q", typ)
		}
	}
	for _, n := range []int{0, -1, 16383} {
		if _, err := decode.ResolveSchemaType("WSTRING", address.Address{Area: "WSTRING", Bit: n}); err == nil {
			t.Errorf("accepted capacity %d", n)
		}
	}
	addr, err := address.Parse("DB10.WSTRING2.256", nil)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := decode.InferAddressType(addr)
	if err != nil || spec.Name != "WSTRING" || spec.StringLen != 256 || spec.SizeBytes != 516 {
		t.Fatalf("infer: %+v, %v", spec, err)
	}
}

func TestWStringWatchTypeResolution(t *testing.T) {
	addr, err := address.Parse("DB1.WSTRING0.10", nil)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := decode.InferAddressType(addr)
	if err != nil {
		t.Fatal(err)
	}
	want := decode.TypeSpec{Name: "WSTRING", StringLen: 10, SizeBytes: 24}
	if direct != want {
		t.Fatalf("direct watch type: %+v, want %+v", direct, want)
	}
	for _, typ := range []string{"WSTRING", "WSTRING[10]"} {
		resolved, err := decode.ResolveSchemaType(typ, addr)
		if err != nil || resolved != direct {
			t.Fatalf("schema watch type %s: %+v, %v", typ, resolved, err)
		}
	}
	for _, n := range []int{0, -1, 16383} {
		if _, err := decode.InferAddressType(address.Address{Area: "WSTRING", Bit: n}); err == nil {
			t.Errorf("inferred invalid capacity %d", n)
		}
	}
}

func TestBareWStringRequiresWStringAddress(t *testing.T) {
	for _, area := range []string{"", "DBX", "X", "DBB", "DBW", "DBD", "STRING", "S"} {
		addr := address.Address{Area: area, Bit: 3}
		if _, err := decode.ResolveSchemaType("WSTRING", addr); err == nil {
			t.Errorf("bare WSTRING accepted area %q with bit/capacity 3", area)
		}
	}
	for _, area := range []string{"DBB", "DBW", "DBD"} {
		spec, err := decode.ResolveSchemaType("WSTRING[10]", address.Address{Area: area})
		if err != nil || spec.StringLen != 10 || spec.SizeBytes != 24 {
			t.Errorf("explicit WSTRING at %s: %+v, %v", area, spec, err)
		}
	}
}

func TestWStringRejectsConflictingCapacity(t *testing.T) {
	for _, n := range []int{1, 10, 20, 16382} {
		addr := address.Address{DB: 1, Area: "WSTRING", Bit: n}
		typ := fmt.Sprintf("WSTRING[%d]", n)
		if _, err := decode.ResolveSchemaType(typ, addr); err != nil {
			t.Fatalf("matching capacity %d: %v", n, err)
		}
		for _, other := range []int{1, 10, 20, 16382} {
			if other == n {
				continue
			}
			addr.Bit = other
			if _, err := decode.ResolveSchemaType(typ, addr); err == nil || !strings.Contains(err.Error(), "does not match address capacity") {
				t.Fatalf("type capacity %d, address capacity %d: %v", n, other, err)
			}
		}
	}
	// Numeric addresses have no capacity suffix; the explicit type remains authoritative.
	if _, err := decode.ResolveSchemaType("WSTRING[10]", address.Address{Area: "DBB"}); err != nil {
		t.Fatal(err)
	}
}

func TestWStringWireFormat(t *testing.T) {
	spec, err := decode.ResolveSchemaType("WSTRING[4]", address.Address{})
	if err != nil {
		t.Fatal(err)
	}
	// ASCII, accented Latin, CJK, and the final BMP code point each occupy one word.
	value := "A\u00e9\u4e2d\uffff"
	want := []byte{0, 4, 0, 4, 0, 0x41, 0, 0xe9, 0x4e, 0x2d, 0xff, 0xff}
	for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		raw, err := decode.EncodeRaw(spec, order, value)
		if err != nil || !bytes.Equal(raw, want) {
			t.Fatalf("encode: %x, %v; want %x", raw, err, want)
		}
		got, err := decode.DecodeRaw(spec, order, raw, 0)
		if err != nil || got != value {
			t.Fatalf("decode: %q, %v", got, err)
		}
	}
	for _, value := range []any{nil, "", "\x00A", "\ufffd"} {
		raw, err := decode.EncodeRaw(spec, binary.BigEndian, value)
		if err != nil {
			t.Fatal(err)
		}
		want := value
		if value == nil {
			want = ""
		}
		got, err := decode.DecodeRaw(spec, binary.BigEndian, raw, 0)
		if err != nil || got != want {
			t.Fatalf("roundtrip %q: %q, %v", want, got, err)
		}
	}
}

func TestWStringCapacityBoundaries(t *testing.T) {
	for _, n := range []int{1, 256, 16382} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			spec, err := decode.ResolveSchemaType(fmt.Sprintf("WSTRING[%d]", n), address.Address{})
			if err != nil {
				t.Fatal(err)
			}
			value := strings.Repeat("\u4e2d", n)
			raw, err := decode.EncodeRaw(spec, binary.LittleEndian, value)
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) != 4+2*n || int(binary.BigEndian.Uint16(raw)) != n || int(binary.BigEndian.Uint16(raw[2:])) != n {
				t.Fatalf("invalid wire size/header: %d, %x", len(raw), raw[:4])
			}
			got, err := decode.DecodeRaw(spec, binary.LittleEndian, raw, 0)
			if err != nil || got != value {
				t.Fatalf("roundtrip failed: %v", err)
			}
			if _, err := decode.EncodeRaw(spec, binary.BigEndian, value+"A"); err == nil {
				t.Fatal("accepted oversized value")
			}
		})
	}
}

func TestWStringRejectsInvalidValues(t *testing.T) {
	spec := decode.TypeSpec{Name: "WSTRING", StringLen: 4, SizeBytes: 12}
	for _, value := range []string{"\xff", "\xc0\x80", "\xed\xa0\x80", "\U0001f600", "\U00010000", "ABCDE"} {
		if _, err := decode.EncodeRaw(spec, binary.BigEndian, value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestWStringRejectsMalformedData(t *testing.T) {
	spec := decode.TypeSpec{Name: "WSTRING", StringLen: 2, SizeBytes: 8}
	for _, raw := range [][]byte{
		nil,
		{0, 2, 0},
		{0, 2, 0, 1, 0, 'A'},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0, 1, 0, 1, 0, 'A', 0, 0},
		{0, 3, 0, 0, 0, 0, 0, 0},
		{0, 2, 0, 3, 0, 0, 0, 0},
		{0, 2, 0, 1, 0xd8, 0, 0, 0},
		{0, 2, 0, 1, 0xdf, 0xff, 0, 0},
		{0, 2, 0, 2, 0xd8, 0x3d, 0xde, 0},
	} {
		if _, err := decode.DecodeRaw(spec, binary.BigEndian, raw, 0); err == nil {
			t.Errorf("accepted malformed data %x", raw)
		}
	}
	// Unused capacity may contain stale data, including surrogate words.
	raw := []byte{0, 2, 0, 1, 0, 'A', 0xd8, 0}
	got, err := decode.DecodeRaw(spec, binary.BigEndian, raw, 0)
	if err != nil || got != "A" {
		t.Fatalf("unused capacity affected decode: %q, %v", got, err)
	}
	for _, invalid := range []decode.TypeSpec{
		{Name: "WSTRING"},
		{Name: "WSTRING", StringLen: -1, SizeBytes: 2},
		{Name: "WSTRING", StringLen: 16383, SizeBytes: 32770},
		{Name: "WSTRING", StringLen: 2, SizeBytes: 4},
	} {
		if _, err := decode.EncodeRaw(invalid, binary.BigEndian, ""); err == nil {
			t.Errorf("accepted invalid encode spec %+v", invalid)
		}
		if _, err := decode.DecodeRaw(invalid, binary.BigEndian, raw, 0); err == nil {
			t.Errorf("accepted invalid decode spec %+v", invalid)
		}
	}
}
