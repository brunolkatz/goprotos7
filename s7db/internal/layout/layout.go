package layout

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

var stringTypeRE = regexp.MustCompile(`^STRING\[(\d+)\]$`)

type TypeSpec struct {
	Name      string
	StringLen int
	SizeBytes int
	BitSize   int
}

func (t TypeSpec) IsBool() bool {
	return t.Name == "BOOL"
}

type PlacedTag struct {
	Tag      schema.Tag
	Address  address.Address
	Type     TypeSpec
	StartBit int
	EndBit   int
}

type Diagnostic struct {
	Level   string
	Message string
}

type Result struct {
	Tags         []PlacedTag
	Diagnostics  []Diagnostic
	ComputedSize int
	FinalSize    int
}

func ParseType(raw string) (TypeSpec, error) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	switch t {
	case "BOOL":
		return TypeSpec{Name: t, SizeBytes: 1, BitSize: 1}, nil
	case "BYTE", "CHAR", "SINT", "USINT":
		return TypeSpec{Name: t, SizeBytes: 1, BitSize: 8}, nil
	case "INT", "UINT", "WORD":
		return TypeSpec{Name: t, SizeBytes: 2, BitSize: 16}, nil
	case "DINT", "UDINT", "DWORD", "REAL", "TIME":
		return TypeSpec{Name: t, SizeBytes: 4, BitSize: 32}, nil
	}
	if m := stringTypeRE.FindStringSubmatch(t); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 254 {
			return TypeSpec{}, fmt.Errorf("invalid STRING length %d", n)
		}
		return TypeSpec{Name: t, StringLen: n, SizeBytes: n + 2, BitSize: (n + 2) * 8}, nil
	}
	return TypeSpec{}, fmt.Errorf("unknown type %q", raw)
}

func Build(s schema.Schema, strict bool) (Result, error) {
	var out Result
	seen := map[int]string{}
	db := s.DB
	maxEnd := 0

	for _, t := range s.Tags {
		addrValue, err := address.Parse(t.Addr, &db)
		if err != nil {
			return Result{}, fmt.Errorf("tag %q: %w", t.Addr, err)
		}
		spec, err := ParseType(t.Type)
		if err != nil {
			return Result{}, fmt.Errorf("tag %s: %w", t.Addr, err)
		}
		if addrValue.Canonical() != t.Addr {
			msg := fmt.Sprintf("non-canonical address %s (canonical %s)", t.Addr, addrValue.Canonical())
			if strict {
				return Result{}, fmt.Errorf("%s", msg)
			}
			out.Diagnostics = append(out.Diagnostics, Diagnostic{Level: "warning", Message: msg})
		}
		if want := expectedArea(spec); want != "" && addrValue.Area != want {
			msg := fmt.Sprintf("type %s usually maps to DB%s addresses, got %s", spec.Name, want, addrValue.Canonical())
			if strict {
				return Result{}, fmt.Errorf("%s", msg)
			}
			out.Diagnostics = append(out.Diagnostics, Diagnostic{Level: "warning", Message: msg})
		}
		start := addrValue.StartBit()
		end := start + spec.BitSize
		for bit := start; bit < end; bit++ {
			if prev, ok := seen[bit]; ok {
				return Result{}, fmt.Errorf("overlapping tags %s and %s", prev, addrValue.Canonical())
			}
			seen[bit] = addrValue.Canonical()
		}
		out.Tags = append(out.Tags, PlacedTag{
			Tag:      t,
			Address:  addrValue,
			Type:     spec,
			StartBit: start,
			EndBit:   end,
		})
		if end > maxEnd {
			maxEnd = end
		}
	}
	out.ComputedSize = (maxEnd + 7) / 8
	out.FinalSize = out.ComputedSize
	if !s.Size.Auto {
		if s.Size.Bytes < out.ComputedSize {
			return Result{}, fmt.Errorf("declared size %d is smaller than required %d", s.Size.Bytes, out.ComputedSize)
		}
		out.FinalSize = s.Size.Bytes
	}
	return out, nil
}

func expectedArea(ts TypeSpec) string {
	switch ts.Name {
	case "BOOL":
		return "X"
	case "BYTE", "CHAR", "SINT", "USINT", "STRING":
		return "B"
	case "INT", "UINT", "WORD":
		return "W"
	case "DINT", "UDINT", "DWORD", "REAL", "TIME":
		return "D"
	}
	if strings.HasPrefix(ts.Name, "STRING[") {
		return "B"
	}
	return ""
}
