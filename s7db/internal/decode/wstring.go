package decode

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

func wstringTypeSpec(n int) (TypeSpec, error) {
	if n < 1 || n > 16382 {
		return TypeSpec{}, fmt.Errorf("invalid WSTRING length %d (must be 1..16382)", n)
	}
	return TypeSpec{Name: "WSTRING", StringLen: n, SizeBytes: 4 + 2*n}, nil
}

func validateWStringSpec(spec TypeSpec) error {
	want, err := wstringTypeSpec(spec.StringLen)
	if err != nil {
		return err
	}
	if spec.SizeBytes != want.SizeBytes {
		return fmt.Errorf("invalid WSTRING type layout: need %d bytes, got %d", want.SizeBytes, spec.SizeBytes)
	}
	return nil
}

func encodeWString(spec TypeSpec, value any) ([]byte, error) {
	if err := validateWStringSpec(spec); err != nil {
		return nil, err
	}
	raw := ""
	if value != nil {
		var err error
		raw, err = asString(value)
		if err != nil {
			return nil, err
		}
	}
	if !utf8.ValidString(raw) {
		return nil, fmt.Errorf("WSTRING value is not valid UTF-8")
	}
	n := 0
	for _, r := range raw {
		if r > 0xffff {
			return nil, fmt.Errorf("WSTRING character U+%X is outside the BMP", r)
		}
		n++
	}
	if n > spec.StringLen {
		return nil, fmt.Errorf("WSTRING length %d exceeds max %d", n, spec.StringLen)
	}
	out := make([]byte, spec.SizeBytes)
	// Siemens WSTRING is big-endian regardless of the schema's numeric byte order.
	binary.BigEndian.PutUint16(out, uint16(spec.StringLen))
	binary.BigEndian.PutUint16(out[2:], uint16(n))
	i := 4
	for _, r := range raw {
		binary.BigEndian.PutUint16(out[i:], uint16(r))
		i += 2
	}
	return out, nil
}

func decodeWString(spec TypeSpec, data []byte) (string, error) {
	if err := validateWStringSpec(spec); err != nil {
		return "", err
	}
	if len(data) < spec.SizeBytes {
		return "", fmt.Errorf("short buffer for WSTRING: need %d bytes, got %d", spec.SizeBytes, len(data))
	}
	max := int(binary.BigEndian.Uint16(data))
	n := int(binary.BigEndian.Uint16(data[2:]))
	if max != spec.StringLen {
		return "", fmt.Errorf("WSTRING header max %d does not match capacity %d", max, spec.StringLen)
	}
	if n > max {
		return "", fmt.Errorf("WSTRING current length %d exceeds max %d", n, max)
	}
	runes := make([]rune, n)
	for i := range runes {
		v := binary.BigEndian.Uint16(data[4+2*i:])
		if v >= 0xd800 && v <= 0xdfff {
			return "", fmt.Errorf("WSTRING contains surrogate U+%04X at character %d", v, i)
		}
		runes[i] = rune(v)
	}
	return string(runes), nil
}
