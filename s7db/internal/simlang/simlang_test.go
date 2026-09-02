package simlang

import (
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

const millScript = `tick 100ms;
var fault_timer : TIME := T#0s;
on OsServiceHeartbeat == true do
  OsServiceHeartbeat := false;
end;
if OsServiceHeartbeat == false then
  fault_timer := fault_timer + tick;
else
  fault_timer := T#0s;
end;
if fault_timer > T#5s then
  OsServiceFault := true;
else
  OsServiceFault := false;
end;
if OsServiceFault then
  BeadWearCalib := false;
  MillSpeed := CleaningSpeed;
else
  MillSpeed := ProdSpeed;
end;
if ValveOpen and Level > 90.0 then
  PumpEnable := false;
end;
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
			{Name: "StatusText", Addr: "DB300.DBB20", Type: "STRING[20]"},
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
	src := "tick 100ms;\non OsServiceHeartbat == true do\nend;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected unknown name diagnostic")
	}
	got := diags[0].String()
	if !strings.Contains(got, "did you mean OsServiceHeartbeat") {
		t.Fatalf("expected suggestion, got: %s", got)
	}
}

func TestPulseBoolAndDefaultWidth(t *testing.T) {
	src := "tick 100ms;\npulse OsServiceFault;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) > 0 {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestPulseWithExplicitWidth(t *testing.T) {
	src := "tick 100ms;\npulse OsServiceFault, 2;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) > 0 {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestOncePulseParsesAndCompiles(t *testing.T) {
	src := "tick 100ms;\nonce pulse OsServiceFault;\nonce pulse OsServiceFault, 2;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) > 0 {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestPulseOnHeartbeatTagIsRejected(t *testing.T) {
	s := testSchema()
	for i := range s.Tags {
		if s.Tags[i].Name == "OsServiceHeartbeat" {
			s.Tags[i].Role = "heartbeat"
		}
	}
	src := "tick 100ms;\npulse OsServiceHeartbeat;\n"
	_, diags := Compile("x.sim", src, s)
	if len(diags) == 0 {
		t.Fatalf("expected heartbeat pulse diag")
	}
	if !strings.Contains(diags[0].String(), "cannot pulse heartbeat tag") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestOncePulseOnHeartbeatTagIsRejected(t *testing.T) {
	s := testSchema()
	for i := range s.Tags {
		if s.Tags[i].Name == "OsServiceHeartbeat" {
			s.Tags[i].Role = "heartbeat"
		}
	}
	src := "tick 100ms;\nonce pulse OsServiceHeartbeat;\n"
	_, diags := Compile("x.sim", src, s)
	if len(diags) == 0 {
		t.Fatalf("expected heartbeat pulse diag")
	}
	if !strings.Contains(diags[0].String(), "cannot pulse heartbeat tag") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestOncePulseRequiresBool(t *testing.T) {
	src := "tick 100ms;\nonce pulse StatusText, 2;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected bool type error")
	}
	if !strings.Contains(diags[0].String(), "pulse requires BOOL") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestPulseMissingSemicolon(t *testing.T) {
	src := "tick 100ms;\npulse OsServiceFault\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected pulse semicolon diag")
	}
	if !strings.Contains(diags[0].String(), `expected ";" after pulse`) {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestOncePulseMissingSemicolon(t *testing.T) {
	src := "tick 100ms;\nonce pulse OsServiceFault, 2\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected pulse semicolon diag")
	}
	if !strings.Contains(diags[0].String(), `expected ";" after pulse`) {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestOnceOnlyPrefixesPulse(t *testing.T) {
	src := "tick 100ms;\nonce AlarmMessage := \"x\";\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(diags[0].String(), `"once" can only prefix pulse`) {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
	if diags[0].Line != 2 {
		t.Fatalf("expected line 2, got %d", diags[0].Line)
	}
}

func TestTypeErrorBoolAssignReal(t *testing.T) {
	src := "tick 100ms;\nOsServiceFault := Level;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected type error")
	}
	if !strings.Contains(diags[0].String(), "cannot assign REAL to BOOL") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestStringVarRequiresLength(t *testing.T) {
	src := "tick 100ms;\nvar Title : STRING := \"x\";\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected STRING length diagnostic")
	}
	if !strings.Contains(diags[0].String(), "STRING requires a max length") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestStringAssignTypeError(t *testing.T) {
	src := "tick 100ms;\nvar Title : STRING[20];\nTitle := 10;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected string assign type error")
	}
	if !strings.Contains(diags[0].String(), "cannot assign INT to STRING[20]") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestStringConcatenationNotSupported(t *testing.T) {
	src := "tick 100ms;\nvar Title : STRING[20] := \"a\";\nTitle := Title + \"x\";\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected concat type error")
	}
	if !strings.Contains(diags[0].String(), "string concatenation not supported") {
		t.Fatalf("unexpected diag: %s", diags[0].String())
	}
}

func TestUnterminatedStringPointsToStringLine(t *testing.T) {
	src := `tick 100ms;
if OSServiceControl == true then
AlarmMessage := "Meu alarme
end
if OSServiceControl == false then
AlarmMessage := "";
end`
	_, diags := Compile(".s7db/sim_scripts/os-service.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected parse diagnostic")
	}
	d := diags[0]
	if d.Line != 3 {
		t.Fatalf("expected line 3, got %d (%s)", d.Line, d.String())
	}
	if d.Col <= 1 {
		t.Fatalf("expected string column, got %d", d.Col)
	}
	if !strings.Contains(d.Snippet, `AlarmMessage := "Meu alarme`) {
		t.Fatalf("unexpected snippet: %q", d.Snippet)
	}
	if strings.Contains(d.String(), "tick 100ms") && strings.Contains(d.String(), "^\n") {
		t.Fatalf("caret should not be on tick line: %s", d.String())
	}
}

func TestClosedStringWithEmptyStringCompiles(t *testing.T) {
	src := `tick 100ms;
if OSServiceControl == true then
AlarmMessage := "Meu alarme";
end
if OSServiceControl == false then
AlarmMessage := "";
end`
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		return
	}
	if strings.Contains(diags[0].String(), "unknown name") {
		return
	}
	t.Fatalf("unexpected parse/type diagnostic: %s", diags[0].String())
}

func TestMissingSemicolonAfterTick(t *testing.T) {
	src := "tick 100ms\nOsServiceFault := false;\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected parse diagnostic")
	}
	if diags[0].Line != 1 {
		t.Fatalf("expected line 1, got %d", diags[0].Line)
	}
	if !strings.Contains(diags[0].Msg, `expected ";" after tick`) {
		t.Fatalf("unexpected message: %s", diags[0].String())
	}
}

func TestUnexpectedSemicolonInIfHeader(t *testing.T) {
	src := "tick 100ms;\nif OSServiceControl == true; then\nAlarmMessage := \"ok\";\nend\n"
	_, diags := Compile("x.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected parse diagnostic")
	}
	if diags[0].Line != 2 {
		t.Fatalf("expected line 2, got %d", diags[0].Line)
	}
	if !strings.Contains(diags[0].Msg, `unexpected ";"`) {
		t.Fatalf("unexpected message: %s", diags[0].String())
	}
}

func TestNoDuplicatedFilePathInParseDiag(t *testing.T) {
	src := "tick 100ms;\nif OSServiceControl == true then\nAlarmMessage := \"x\";\nend;\n"
	_, diags := Compile("test.sim", src, testSchema())
	if len(diags) == 0 {
		t.Fatalf("expected diagnostic for unknown name")
	}
	msg := diags[0].String()
	if strings.Contains(msg, "test.sim:1:1: parse error: test.sim:") {
		t.Fatalf("duplicated path in diagnostic: %s", msg)
	}
}
