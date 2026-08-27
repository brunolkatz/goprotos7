package decode

import (
	"encoding/binary"
	"fmt"
	"math"
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

func DecodeRaw(spec TypeSpec, endian binary.ByteOrder, data []byte, bit int) (any, error) {
	if spec.Name == "BOOL" {
		if len(data) < 1 {
			return nil, fmt.Errorf("short buffer for BOOL")
		}
		if bit < 0 || bit > 7 {
			return nil, fmt.Errorf("invalid bit %d", bit)
		}
		return data[0]&(1<<bit) != 0, nil
	}
	if len(data) < spec.SizeBytes {
		return nil, fmt.Errorf("short buffer: need %d bytes, got %d", spec.SizeBytes, len(data))
	}
	switch spec.Name {
	case "BYTE", "USINT", "CHAR":
		return uint8(data[0]), nil
	case "SINT":
		return int8(data[0]), nil
	case "INT":
		return int16(endian.Uint16(data[:2])), nil
	case "UINT", "WORD":
		return endian.Uint16(data[:2]), nil
	case "DINT":
		return int32(endian.Uint32(data[:4])), nil
	case "UDINT", "DWORD", "TIME":
		return endian.Uint32(data[:4]), nil
	case "REAL":
		return math.Float32frombits(endian.Uint32(data[:4])), nil
	case "STRING":
		if len(data) < 2 {
			return "", nil
		}
		max := int(data[0])
		if spec.StringLen > 0 && max > spec.StringLen {
			max = spec.StringLen
		}
		if max > len(data)-2 {
			max = len(data) - 2
		}
		n := int(data[1])
		if n > max {
			n = max
		}
		if n < 0 {
			n = 0
		}
		return string(data[2 : 2+n]), nil
	default:
		return nil, fmt.Errorf("unsupported type %q", spec.Name)
	}
}

func EncodeRaw(spec TypeSpec, endian binary.ByteOrder, value any) ([]byte, error) {
	switch spec.Name {
	case "BYTE", "USINT", "CHAR":
		v, err := asUint(value)
		if err != nil {
			return nil, err
		}
		return []byte{byte(v)}, nil
	case "SINT":
		v, err := asInt(value)
		if err != nil {
			return nil, err
		}
		return []byte{byte(int8(v))}, nil
	case "INT":
		v, err := asInt(value)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 2)
		endian.PutUint16(out, uint16(int16(v)))
		return out, nil
	case "UINT", "WORD":
		v, err := asUint(value)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 2)
		endian.PutUint16(out, uint16(v))
		return out, nil
	case "DINT":
		v, err := asInt(value)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 4)
		endian.PutUint32(out, uint32(int32(v)))
		return out, nil
	case "UDINT", "DWORD", "TIME":
		v, err := asUint(value)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 4)
		endian.PutUint32(out, uint32(v))
		return out, nil
	case "REAL":
		v, err := asFloat(value)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 4)
		endian.PutUint32(out, math.Float32bits(float32(v)))
		return out, nil
	case "STRING":
		s, err := asString(value)
		if err != nil {
			return nil, err
		}
		maxLen := spec.StringLen
		if maxLen == 0 {
			maxLen = len(s)
		}
		if len(s) > maxLen {
			return nil, fmt.Errorf("string length %d exceeds max %d", len(s), maxLen)
		}
		size := spec.SizeBytes
		if size == 0 {
			size = maxLen + 2
		}
		out := make([]byte, size)
		out[0] = byte(maxLen)
		out[1] = byte(len(s))
		copy(out[2:], []byte(s))
		return out, nil
	case "BOOL":
		v, err := asBool(value)
		if err != nil {
			return nil, err
		}
		if v {
			return []byte{1}, nil
		}
		return []byte{0}, nil
	default:
		return nil, fmt.Errorf("unsupported type %q", spec.Name)
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
		if n < 1 || n > 16382 {
			return TypeSpec{}, fmt.Errorf("invalid STRING length %d", n)
		}
		return TypeSpec{Name: "STRING", StringLen: n, SizeBytes: n + 2}, nil
	}
	return TypeSpec{}, fmt.Errorf("unknown type %q", typ)
}

func asBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(x))
		if err != nil {
			return false, fmt.Errorf("invalid bool %q", x)
		}
		return b, nil
	case float64:
		return x != 0, nil
	case int:
		return x != 0, nil
	default:
		return false, fmt.Errorf("invalid bool value %T", v)
	}
}

func asInt(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int8:
		return int64(x), nil
	case int16:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case uint:
		return int64(x), nil
	case uint16:
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case uint64:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid integer %q", x)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("invalid integer value %T", v)
	}
}

func asUint(v any) (uint64, error) {
	switch x := v.(type) {
	case int:
		return uint64(x), nil
	case int64:
		return uint64(x), nil
	case uint:
		return uint64(x), nil
	case uint16:
		return uint64(x), nil
	case uint32:
		return uint64(x), nil
	case uint64:
		return x, nil
	case float64:
		return uint64(x), nil
	case string:
		i, err := strconv.ParseUint(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid unsigned integer %q", x)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("invalid unsigned integer value %T", v)
	}
}

func asFloat(v any) (float64, error) {
	switch x := v.(type) {
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case uint64:
		return float64(x), nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid float %q", x)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("invalid float value %T", v)
	}
}

func asString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}
