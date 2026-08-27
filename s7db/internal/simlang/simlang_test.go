package simlang

import (
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

const millScript = `tick 100ms
var fault_timer : TIME := T#0s
on OsServiceHeartbeat == true do
  OsServiceHeartbeat := false
end
if OsServiceHeartbeat == false then
  fault_timer := fault_timer + tick
else
  fault_timer := T#0s
end
if fault_timer > T#5s then
  OsServiceFault := true
else
  OsServiceFault := false
end
if OsServiceFault then
  BeadWearCalib := false
  MillSpeed := CleaningSpeed
else
  MillSpeed := ProdSpeed
end
if ValveOpen and Level > 90.0 then
  PumpEnable := false
end
`

func testSchema() schema.Schema {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "OsServiceHeartbeat", Addr: "DB300.DBX0.0", Type: "BOOL"},
			{Name: "OsServiceFault", Addr: "DB300.DBX0.1", Type: "BOOL"},
			{Name: "BeadWearCalib", Addr: "DB300.DBX0.2", Type: "BOOL"},
			{Name: "MillSpeed", Addr: "DB300.DBD4", Type: "REAL"},
			{Name: "CleaningSpeed", Addr: "DB300.DBD8", Type: "REAL"},
			{Name: "ProdSpeed", Addr: "DB300.DBD12", Type: "REAL"},
			{Name: "ValveOpen", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "Level", Addr: "DB300.DBD16", Type: "REAL"},
			{Name: "PumpEnable", Addr: "DB300.DBX1.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	return s
}

func TestParseAndCompileMill(t *testing.T) {
	prog, diags := Compile("mill.sim", millScript, testSchema())
	if len(diags) > 0 {
		t.Fatalf("unexpected compile diags: %v", diags[0].String())
	}
	if prog.Tick.String() != "100ms" {
		t.Fatalf("tick mismatch: %s", prog.Tick)
	}
}

func TestUnknownNameSuggestion(t *testing.T) {
	src := "tick 100ms\non OsServiceHeartbat == true do\nend\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected unknown name diagnostic")
	}
	got := diags[0].String()
	if !strings.Contains(got, "did you mean OsServiceHeartbeat") {
		t.Fatalf("expected suggestion, got: %s", got)
	}
}

func TestTypeErrorBoolAssignReal(t *testing.T) {
	src := "tick 100ms\nOsServiceFault := Level\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected type error")
	}
	if !strings.Contains(diags[0].String(), "cannot assign REAL to BOOL") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}
