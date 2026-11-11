package simlang

import (
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

func TestWStringSchemaValidation(t *testing.T) {
	tests := []struct {
		name string
		addr string
		typ  string
		init any
		want string
	}{
		{"overflow", "DB1.DBB0", "WSTRING[2]", "abc", "exceeds max"},
		{"non BMP", "DB1.DBB0", "WSTRING[2]", "\U0001f600", "outside the BMP"},
		{"invalid UTF8", "DB1.DBB0", "WSTRING[2]", "\xff", "not valid UTF-8"},
		{"capacity mismatch", "DB1.WSTRING0.10", "WSTRING[2]", nil, "does not match address capacity"},
		{"short capacity mismatch", "WSTRING0.10", "WSTRING[2]", nil, "does not match address capacity"},
		{"bare at byte address", "DB1.DBB0", "WSTRING", nil, "requires a WSTRING address"},
		{"invalid address", "bad", "WSTRING[2]", nil, "schema tag WideStatus"},
		{"empty", "DB1.WSTRING0.2", "WSTRING", nil, ""},
		{"matching explicit", "WSTRING0.2", "WSTRING[2]", "\u00e9\u4e2d", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := schema.Schema{DB: 1, Tags: []schema.Tag{
				{Name: "WideStatus", Addr: tt.addr, Type: tt.typ, Init: tt.init},
			}}
			prog, diags := Compile("wide.sim", "tick 100ms;\n", s)
			if tt.want == "" {
				if len(diags) != 0 || prog == nil {
					t.Fatalf("unexpected diagnostics: %v", diags)
				}
				return
			}
			if prog != nil || len(diags) == 0 || !strings.Contains(diags[0].Msg, tt.want) {
				t.Fatalf("expected %q, got program=%v diagnostics=%v", tt.want, prog, diags)
			}
		})
	}
}
