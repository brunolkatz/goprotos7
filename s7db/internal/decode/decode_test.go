package decode_test

import (
	"encoding/binary"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/decode"
)

func TestDecodeBoolBits(t *testing.T) {
	raw := []byte{0b10100101}
	spec, err := decode.ResolveSchemaType("BOOL", address.Address{})
	if err != nil {
		t.Fatalf("resolve type failed: %v", err)
	}
	for bit := 0; bit < 8; bit++ {
		got, err := decode.DecodeRaw(spec, binary.BigEndian, raw, bit)
		if err != nil {
			t.Fatalf("bit %d: %v", bit, err)
		}
		want := raw[0]&(1<<bit) != 0
		if got != want {
			t.Fatalf("bit %d: got %v, want %v", bit, got, want)
		}
	}
}

func TestDecodeByte(t *testing.T) {
	spec, err := decode.ResolveSchemaType("BYTE", address.Address{})
	if err != nil {
		t.Fatalf("resolve type failed: %v", err)
	}
	got, err := decode.DecodeRaw(spec, binary.BigEndian, []byte{0xFA}, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got != uint8(250) {
		t.Fatalf("got %v, want 250", got)
	}
}

func TestDecodeWordBigEndian(t *testing.T) {
	spec, err := decode.ResolveSchemaType("WORD", address.Address{})
	if err != nil {
		t.Fatalf("resolve type failed: %v", err)
	}
	got, err := decode.DecodeRaw(spec, binary.BigEndian, []byte{0x12, 0x34}, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got != uint16(0x1234) {
		t.Fatalf("got %v, want %d", got, uint16(0x1234))
	}
}

func TestDecodeDWord(t *testing.T) {
	spec, err := decode.ResolveSchemaType("DWORD", address.Address{})
	if err != nil {
		t.Fatalf("resolve type failed: %v", err)
	}
	got, err := decode.DecodeRaw(spec, binary.BigEndian, []byte{0x41, 0x48, 0x00, 0x00}, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got != uint32(1095237632) {
		t.Fatalf("got %v, want %d", got, uint32(1095237632))
	}
}

func TestDecodeReal(t *testing.T) {
	spec, err := decode.ResolveSchemaType("REAL", address.Address{})
	if err != nil {
		t.Fatalf("resolve type failed: %v", err)
	}
	got, err := decode.DecodeRaw(spec, binary.BigEndian, []byte{0x41, 0x48, 0x00, 0x00}, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got != float32(12.5) {
		t.Fatalf("got %v, want 12.5", got)
	}
}

func TestDecodeString(t *testing.T) {
	spec, err := decode.ResolveSchemaType("STRING[8]", address.Address{})
	if err != nil {
		t.Fatalf("resolve type failed: %v", err)
	}
	got, err := decode.DecodeRaw(spec, binary.BigEndian, []byte{8, 5, 'H', 'e', 'l', 'l', 'o', 0, 0, 0}, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got != "Hello" {
		t.Fatalf("got %q, want %q", got, "Hello")
	}
}

func TestInferAddressType(t *testing.T) {
	addrValue, err := address.Parse("DB10.DBW2", nil)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	spec, err := decode.InferAddressType(addrValue)
	if err != nil {
		t.Fatalf("infer failed: %v", err)
	}
	if spec.Name != "WORD" || spec.SizeBytes != 2 {
		t.Fatalf("got %+v, want WORD size 2", spec)
	}
}
