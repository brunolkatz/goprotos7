package sim

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseDuration(raw string) (time.Duration, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "T#")
	s = strings.ReplaceAll(s, "US", "µs")
	return time.ParseDuration(strings.ToLower(s))
}

func compare(a, b any, op string) (bool, error) {
	if ab, ok := a.(bool); ok {
		bb, ok := b.(bool)
		if !ok {
			return false, fmt.Errorf("cannot compare BOOL with %T", b)
		}
		switch op {
		case "==":
			return ab == bb, nil
		case "!=":
			return ab != bb, nil
		default:
			return false, fmt.Errorf("invalid bool operator %s", op)
		}
	}
	af, ak := asNumeric(a)
	bf, bk := asNumeric(b)
	if ak && bk {
		switch op {
		case "==":
			return af == bf, nil
		case "!=":
			return af != bf, nil
		case ">":
			return af > bf, nil
		case ">=":
			return af >= bf, nil
		case "<":
			return af < bf, nil
		case "<=":
			return af <= bf, nil
		}
	}
	at, aok := a.(time.Duration)
	bt, bok := b.(time.Duration)
	if aok && bok {
		switch op {
		case "==":
			return at == bt, nil
		case "!=":
			return at != bt, nil
		case ">":
			return at > bt, nil
		case ">=":
			return at >= bt, nil
		case "<":
			return at < bt, nil
		case "<=":
			return at <= bt, nil
		}
	}
	return false, fmt.Errorf("incompatible compare %T %s %T", a, op, b)
}

func addsub(a, b any, op string) (any, error) {
	if at, ok := a.(time.Duration); ok {
		bt, ok := b.(time.Duration)
		if !ok {
			return nil, fmt.Errorf("TIME operation requires TIME operands")
		}
		if op == "+" {
			return at + bt, nil
		}
		return at - bt, nil
	}
	if isInt(a) && isInt(b) {
		ai, _ := toInt(a)
		bi, _ := toInt(b)
		if op == "+" {
			return ai + bi, nil
		}
		return ai - bi, nil
	}
	af, ak := asNumeric(a)
	bf, bk := asNumeric(b)
	if ak && bk {
		if op == "+" {
			return af + bf, nil
		}
		return af - bf, nil
	}
	return nil, fmt.Errorf("invalid operands for %s: %T and %T", op, a, b)
}

func muldiv(a, b any, op string) (any, error) {
	if isInt(a) && isInt(b) {
		ai, _ := toInt(a)
		bi, _ := toInt(b)
		if op == "/" && bi == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		if op == "*" {
			return ai * bi, nil
		}
		return ai / bi, nil
	}
	af, ak := asNumeric(a)
	bf, bk := asNumeric(b)
	if ak && bk {
		if op == "/" && bf == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		if op == "*" {
			return af * bf, nil
		}
		return af / bf, nil
	}
	return nil, fmt.Errorf("invalid operands for %s: %T and %T", op, a, b)
}

func isInt(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64:
		return true
	default:
		return false
	}
}

func toInt(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	default:
		return 0, false
	}
}

func asNumeric(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func equal(a, b any) bool {
	switch x := a.(type) {
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case int64:
		y, ok := toInt(b)
		return ok && x == y
	case float64:
		y, ok := asNumeric(b)
		return ok && x == y
	case time.Duration:
		y, ok := b.(time.Duration)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}
