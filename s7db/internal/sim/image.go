package sim

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
)

type Image struct {
	values          map[string]any
	types           map[string]simlang.ValueType
	stringMaxLen    map[string]int
	stringTruncated map[string]bool
	tags            map[string]schema.Tag
}

func NewImageFromSchema(s schema.Schema, prog *simlang.Program) *Image {
	img := &Image{
		values:          map[string]any{},
		types:           map[string]simlang.ValueType{},
		stringMaxLen:    map[string]int{},
		stringTruncated: map[string]bool{},
		tags:            map[string]schema.Tag{},
	}
	for name, sym := range prog.Symbols {
		img.types[name] = sym.Type
		if sym.Type == simlang.TypeString && sym.StringLen > 0 {
			img.stringMaxLen[name] = sym.StringLen
		}
	}
	for _, t := range s.Tags {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		img.tags[name] = t
		if sym, ok := prog.Symbols[name]; ok && sym.IsTag {
			img.values[name] = coerceInit(sym, t.Init)
		}
	}
	for name, typ := range img.types {
		if _, ok := img.values[name]; ok {
			continue
		}
		img.values[name] = zeroValue(typ)
	}
	return img
}

func (i *Image) Get(name string) (any, bool) {
	v, ok := i.values[name]
	return v, ok
}

func (i *Image) Set(name string, v any) error {
	t, ok := i.types[name]
	if !ok {
		return fmt.Errorf("unknown name %q", name)
	}
	c, err := coerceValue(t, v)
	if err != nil {
		return err
	}
	if t == simlang.TypeString {
		s, _ := c.(string)
		if maxLen := i.stringMaxLen[name]; maxLen > 0 && len(s) > maxLen {
			i.stringTruncated[name] = true
			s = s[:maxLen]
		}
		c = s
	}
	i.values[name] = c
	return nil
}

func (i *Image) Type(name string) (simlang.ValueType, bool) {
	t, ok := i.types[name]
	return t, ok
}

func (i *Image) Tag(name string) (schema.Tag, bool) {
	t, ok := i.tags[name]
	return t, ok
}

func (i *Image) Snapshot() map[string]any {
	out := make(map[string]any, len(i.values))
	for k, v := range i.values {
		out[k] = v
	}
	return out
}

func (i *Image) ConsumeStringTruncated(name string) bool {
	if !i.stringTruncated[name] {
		return false
	}
	delete(i.stringTruncated, name)
	return true
}

func zeroValue(t simlang.ValueType) any {
	switch t {
	case simlang.TypeBool:
		return false
	case simlang.TypeInt:
		return int64(0)
	case simlang.TypeReal:
		return float64(0)
	case simlang.TypeTime:
		return time.Duration(0)
	case simlang.TypeString:
		return ""
	default:
		return nil
	}
}

func coerceInit(sym simlang.Symbol, v any) any {
	c, err := coerceValue(sym.Type, v)
	if err != nil {
		return zeroValue(sym.Type)
	}
	if sym.Type == simlang.TypeString && sym.StringLen > 0 {
		s := fmt.Sprintf("%v", c)
		if len(s) > sym.StringLen {
			s = s[:sym.StringLen]
		}
		return s
	}
	return c
}

func coerceValue(t simlang.ValueType, v any) (any, error) {
	switch t {
	case simlang.TypeBool:
		return asBool(v)
	case simlang.TypeInt:
		return asInt(v)
	case simlang.TypeReal:
		return asFloat(v)
	case simlang.TypeTime:
		return asDuration(v)
	case simlang.TypeString:
		return fmt.Sprintf("%v", v), nil
	default:
		return nil, fmt.Errorf("unsupported type %s", t)
	}
}

func asBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(x))
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%v", x) != "0", nil
	case float64:
		return x != 0, nil
	default:
		return false, fmt.Errorf("invalid BOOL value %T", v)
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
		return strconv.ParseInt(strings.TrimSpace(x), 10, 64)
	default:
		return 0, fmt.Errorf("invalid INT value %T", v)
	}
}

func asFloat(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(x), 64)
	default:
		return 0, fmt.Errorf("invalid REAL value %T", v)
	}
}

func asDuration(v any) (time.Duration, error) {
	switch x := v.(type) {
	case time.Duration:
		return x, nil
	case int:
		return time.Duration(x) * time.Millisecond, nil
	case int64:
		return time.Duration(x) * time.Millisecond, nil
	case float64:
		return time.Duration(x) * time.Millisecond, nil
	case uint32:
		return time.Duration(x) * time.Millisecond, nil
	case string:
		raw := strings.ToUpper(strings.TrimSpace(x))
		raw = strings.TrimPrefix(raw, "T#")
		raw = strings.ReplaceAll(raw, "US", "µs")
		return time.ParseDuration(strings.ToLower(raw))
	default:
		return 0, fmt.Errorf("invalid TIME value %T", v)
	}
}
