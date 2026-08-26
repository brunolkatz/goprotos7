package decode

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
)

var stringTypeRE = regexp.MustCompile(`^STRING\[(\d+)\]$`)

type TypeSpec struct {
	Name      string
	SizeBytes int
	StringLen int
}

func ByteOrder(raw string) (binary.ByteOrder, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "big":
		return binary.BigEndian, nil
	case "little":
		return binary.LittleEndian, nil
	default:
		return nil, fmt.Errorf("invalid endian %q", raw)
	}
}

func ResolveSchemaType(typ string, addr address.Address) (TypeSpec, error) {
	return parseTypeSpec(typ, &addr)
}

func InferAddressType(addr address.Address) (TypeSpec, error) {
	switch strings.ToUpper(strings.TrimSpace(addr.Area)) {
	case "X", "DBX":
		return TypeSpec{Name: "BOOL", SizeBytes: 1}, nil
	case "B", "DBB":
		return TypeSpec{Name: "BYTE", SizeBytes: 1}, nil
	case "W", "DBW":
		return TypeSpec{Name: "WORD", SizeBytes: 2}, nil
	case "D", "DBD":
		return TypeSpec{Name: "DWORD", SizeBytes: 4}, nil
	default:
		return TypeSpec{}, fmt.Errorf("cannot infer type from address %s", addr.Canonical())
	}
}

func Decode(data any) (any, string, error) {
	if data == nil {
		return nil, "", fmt.Errorf("data is nil")
	}
	switch reflect.TypeOf(data).Kind() {
	case reflect.Int8:
		return data.(int8), "SINT", nil
	case reflect.Uint8:
		return data.(uint8), "USINT", nil
	case reflect.Int16:
		return data.(int16), "INT", nil
	case reflect.Int64:
		return data.(int64), "LINT", nil
	case reflect.Int:
		return data.(int), "INT", nil
	case reflect.Uint16:
		return data.(uint16), "UINT", nil
	case reflect.Int32:
		return data.(int32), "DINT", nil
	case reflect.Uint32:
		return data.(uint32), "UDINT", nil
	case reflect.Uint64:
		return data.(uint64), "ULINT", nil
	case reflect.Float32:
		return data.(float32), "REAL", nil
	case reflect.Float64:
		return data.(float64), "LREAL", nil
	case reflect.Bool:
		return data.(bool), "BOOL", nil
	case reflect.String:
		return data.(string), "STRING", nil
	default:
		return nil, "", fmt.Errorf("unsupported data type %T", data)
	}
}

func parseTypeSpec(typ string, addr *address.Address) (TypeSpec, error) {
	t := strings.ToUpper(strings.TrimSpace(typ))
	switch t {
	case "BOOL":
		return TypeSpec{Name: "BOOL", SizeBytes: 1}, nil
	case "BYTE", "USINT", "CHAR", "SINT":
		return TypeSpec{Name: t, SizeBytes: 1}, nil
	case "INT", "UINT", "WORD":
		return TypeSpec{Name: t, SizeBytes: 2}, nil
	case "DINT", "UDINT", "DWORD", "REAL", "TIME":
		return TypeSpec{Name: t, SizeBytes: 4}, nil
	case "STRING":
		if addr == nil {
			return TypeSpec{Name: "STRING"}, nil
		}
		if addr.Bit < 1 || addr.Bit > 16382 {
			return TypeSpec{}, fmt.Errorf("STRING requires length in address suffix (1..16382)")
		}
		return TypeSpec{Name: "STRING", StringLen: addr.Bit, SizeBytes: addr.Bit + 2}, nil
	}
	if m := stringTypeRE.FindStringSubmatch(t); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 254 {
			return TypeSpec{}, fmt.Errorf("invalid STRING length %d", n)
		}
		return TypeSpec{Name: "STRING", StringLen: n, SizeBytes: n + 2}, nil
	}
	return TypeSpec{}, fmt.Errorf("unknown type %q", typ)
}
