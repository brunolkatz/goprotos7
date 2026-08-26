package pack

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/brunolkatz/goprotos7/s7db/internal/layout"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

func Pack(s schema.Schema, fill byte, pad int, strict bool) ([]byte, int, error) {
	l, err := layout.Build(s, strict)
	if err != nil {
		return nil, 0, err
	}
	out := make([]byte, l.FinalSize)
	for i := range out {
		out[i] = fill
	}
	order, err := byteOrder(s.Endian)
	if err != nil {
		return nil, 0, err
	}
	for _, p := range l.Tags {
		byteOffset := p.StartBit / 8
		if p.Type.IsBool() {
			v, err := asBool(p.Tag.Init)
			if err != nil {
				return nil, 0, fmt.Errorf("tag %s: %w", p.Tag.Addr, err)
			}
			mask := byte(1 << p.Address.Bit)
			if v {
				out[byteOffset] |= mask
			} else {
				out[byteOffset] &^= mask
			}
			continue
		}
		enc, err := encodeValue(p.Type, p.Tag.Init, order)
		if err != nil {
			return nil, 0, fmt.Errorf("tag %s: %w", p.Tag.Addr, err)
		}
		if byteOffset+len(enc) > len(out) {
			return nil, 0, fmt.Errorf("tag %s out of range", p.Tag.Addr)
		}
		copy(out[byteOffset:], enc)
	}
	if pad > 0 && len(out)%pad != 0 {
		target := ((len(out) + pad - 1) / pad) * pad
		padded := make([]byte, target)
		copy(padded, out)
		for i := len(out); i < len(padded); i++ {
			padded[i] = fill
		}
		out = padded
	}
	return out, l.ComputedSize, nil
}

func Unpack(data []byte, db int, template *schema.Schema) (schema.Schema, error) {
	if template == nil {
		tags := make([]schema.Tag, 0, len(data))
		for i, b := range data {
			tags = append(tags, schema.Tag{
				Addr: fmt.Sprintf("DB%d.DBB%d", db, i),
				Type: "BYTE",
				Init: int(b),
			})
		}
		return schema.Schema{
			Version: 1,
			DB:      db,
			Endian:  "big",
			Size:    schema.SizeSpec{Auto: false, Bytes: len(data)},
			Tags:    tags,
		}, nil
	}
	out := *template
	order, err := byteOrder(out.Endian)
	if err != nil {
		return schema.Schema{}, err
	}
	l, err := layout.Build(out, false)
	if err != nil {
		return schema.Schema{}, err
	}
	tags := make([]schema.Tag, 0, len(l.Tags))
	for _, p := range l.Tags {
		t := p.Tag
		offset := p.StartBit / 8
		if p.Type.IsBool() {
			if offset >= len(data) {
				t.Init = false
			} else {
				t.Init = (data[offset]&(1<<p.Address.Bit) != 0)
			}
			tags = append(tags, t)
			continue
		}
		if offset+p.Type.SizeBytes > len(data) {
			t.Init = nil
			tags = append(tags, t)
			continue
		}
		v, err := decodeValue(data[offset:offset+p.Type.SizeBytes], p.Type, order)
		if err != nil {
			t.Init = nil
		} else {
			t.Init = v
		}
		tags = append(tags, t)
	}
	out.Tags = tags
	out.Size = schema.SizeSpec{Auto: false, Bytes: len(data)}
	return out, nil
}

func byteOrder(raw string) (binary.ByteOrder, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "big":
		return binary.BigEndian, nil
	case "little":
		return binary.LittleEndian, nil
	default:
		return nil, fmt.Errorf("invalid endian %q", raw)
	}
}

func encodeValue(ts layout.TypeSpec, init any, order binary.ByteOrder) ([]byte, error) {
	switch ts.Name {
	case "BYTE", "CHAR", "USINT":
		v, err := asUint64(init)
		if err != nil {
			return nil, err
		}
		return []byte{byte(v)}, nil
	case "SINT":
		v, err := asInt64(init)
		if err != nil {
			return nil, err
		}
		return []byte{byte(int8(v))}, nil
	case "INT":
		v, err := asInt64(init)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 2)
		order.PutUint16(out, uint16(int16(v)))
		return out, nil
	case "UINT", "WORD":
		v, err := asUint64(init)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 2)
		order.PutUint16(out, uint16(v))
		return out, nil
	case "DINT", "TIME":
		v, err := asInt64(init)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 4)
		order.PutUint32(out, uint32(int32(v)))
		return out, nil
	case "UDINT", "DWORD":
		v, err := asUint64(init)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 4)
		order.PutUint32(out, uint32(v))
		return out, nil
	case "REAL":
		v, err := asFloat64(init)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 4)
		order.PutUint32(out, math.Float32bits(float32(v)))
		return out, nil
	case "STRING":
		raw, err := asString(init)
		if err != nil {
			return nil, err
		}
		out := make([]byte, ts.SizeBytes)
		out[0] = byte(ts.StringLen)
		if len(raw) > ts.StringLen {
			raw = raw[:ts.StringLen]
		}
		out[1] = byte(len(raw))
		copy(out[2:], []byte(raw))
		return out, nil
	}
	if strings.HasPrefix(ts.Name, "STRING[") {
		raw, err := asString(init)
		if err != nil {
			return nil, err
		}
		if len(raw) > ts.StringLen {
			raw = raw[:ts.StringLen]
		}
		out := make([]byte, ts.SizeBytes)
		out[0] = byte(ts.StringLen)
		out[1] = byte(len(raw))
		copy(out[2:], []byte(raw))
		return out, nil
	}
	return nil, fmt.Errorf("unsupported type %s", ts.Name)
}

func decodeValue(raw []byte, ts layout.TypeSpec, order binary.ByteOrder) (any, error) {
	switch ts.Name {
	case "BYTE", "CHAR", "USINT":
		return int(raw[0]), nil
	case "SINT":
		return int(int8(raw[0])), nil
	case "INT":
		return int(int16(order.Uint16(raw))), nil
	case "UINT", "WORD":
		return int(order.Uint16(raw)), nil
	case "DINT", "TIME":
		return int64(int32(order.Uint32(raw))), nil
	case "UDINT", "DWORD":
		return uint64(order.Uint32(raw)), nil
	case "REAL":
		return math.Float32frombits(order.Uint32(raw)), nil
	case "STRING":
		if len(raw) < 2 {
			return "", nil
		}
		length := int(raw[1])
		max := int(raw[0])
		if max > len(raw)-2 {
			max = len(raw) - 2
		}
		if length > max {
			length = max
		}
		return string(raw[2 : 2+length]), nil
	}
	if strings.HasPrefix(ts.Name, "STRING[") {
		if len(raw) < 2 {
			return "", nil
		}
		length := int(raw[1])
		max := int(raw[0])
		if max > len(raw)-2 {
			max = len(raw) - 2
		}
		if length > max {
			length = max
		}
		return string(raw[2 : 2+length]), nil
	}
	return nil, fmt.Errorf("unsupported type %s", ts.Name)
}

func asBool(v any) (bool, error) {
	if v == nil {
		return false, nil
	}
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(x))
		if err != nil {
			return false, fmt.Errorf("invalid bool %q", x)
		}
		return b, nil
	case int:
		return x != 0, nil
	case int64:
		return x != 0, nil
	case float64:
		return x != 0, nil
	default:
		return false, fmt.Errorf("invalid bool value %T", v)
	}
}

func asInt64(v any) (int64, error) {
	if v == nil {
		return 0, nil
	}
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
	case uint64:
		return int64(x), nil
	case float32:
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

func asUint64(v any) (uint64, error) {
	if v == nil {
		return 0, nil
	}
	switch x := v.(type) {
	case int:
		return uint64(x), nil
	case int64:
		return uint64(x), nil
	case uint:
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

func asFloat64(v any) (float64, error) {
	if v == nil {
		return 0, nil
	}
	switch x := v.(type) {
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int64:
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
	if v == nil {
		return "", nil
	}
	switch x := v.(type) {
	case string:
		return x, nil
	default:
		return fmt.Sprintf("%v", x), nil
	}
}
