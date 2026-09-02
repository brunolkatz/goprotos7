package sim

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/decode"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
)

func schemaForSim() schema.Schema {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "OsServiceHeartbeat", Addr: "DB300.DBX0.0", Type: "BOOL"},
			{Name: "OsServiceFault", Addr: "DB300.DBX0.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	return s
}

func TestOfflineHeartbeatClears(t *testing.T) {
	src := "tick 100ms;\non OsServiceHeartbeat == true do\nOsServiceHeartbeat := false;\nend;\n"
	prog, ds := simlang.Compile("a.sim", src, schemaForSim())
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(schemaForSim(), prog)
	_ = img.Set("OsServiceHeartbeat", true)
	r := NewRunner(prog, img)
	if _, err := r.Step(); err != nil {
		t.Fatalf("step failed: %v", err)
	}
	v, _ := img.Get("OsServiceHeartbeat")
	if b, _ := v.(bool); b {
		t.Fatalf("expected heartbeat to be false after step")
	}
}

func TestOfflineFaultAfterTimeout(t *testing.T) {
	src := `tick 100ms;
var fault_timer : TIME := T#0s;
if OsServiceHeartbeat == false then
  fault_timer := fault_timer + tick;
else
  fault_timer := T#0s;
end
if fault_timer > T#5s then
  OsServiceFault := true;
else
  OsServiceFault := false;
end`
	prog, ds := simlang.Compile("a.sim", src, schemaForSim())
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(schemaForSim(), prog)
	_ = img.Set("OsServiceHeartbeat", false)
	r := NewRunner(prog, img)
	for i := 0; i < 51; i++ {
		if _, err := r.Step(); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	v, _ := img.Get("OsServiceFault")
	if b, _ := v.(bool); !b {
		t.Fatalf("expected fault true after timeout")
	}
}

func TestDivByZeroRuntime(t *testing.T) {
	src := "tick 100ms;\nvar x : REAL := 1.0;\nx := x / 0;\n"
	prog, ds := simlang.Compile("a.sim", src, schemaForSim())
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(schemaForSim(), prog)
	r := NewRunner(prog, img)
	_, err := r.Step()
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("expected division by zero runtime error, got %v", err)
	}
}

type pushSpy struct {
	pulls int
	pushs int
	last  []Change
}

func (p *pushSpy) Pull(context.Context, []string) error { p.pulls++; return nil }
func (p *pushSpy) Push(_ context.Context, c []Change) error {
	p.pushs++
	p.last = append([]Change(nil), c...)
	return nil
}

func TestSinkReadOnlyNoPush(t *testing.T) {
	_ = time.Second
	spy := &pushSpy{}
	if err := spy.Pull(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if spy.pushs != 0 {
		t.Fatalf("expected no push calls")
	}
}

func TestPulseActsForTwoTicksThenClears(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "Trigger", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "PopupOKActivationPulse", Addr: "DB300.DBX1.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\non rising Trigger do\n  pulse PopupOKActivationPulse;\nend;\n"
	prog, ds := simlang.Compile("pulse.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	_ = img.Set("Trigger", true)
	r := NewRunner(prog, img)
	for i := 0; i < 3; i++ {
		if _, err := r.Step(); err != nil {
			t.Fatalf("step %d failed: %v", i+1, err)
		}
	}
	v, _ := img.Get("PopupOKActivationPulse")
	if b, _ := v.(bool); b {
		t.Fatalf("expected pulse to clear after two ticks")
	}
}

func TestOncePulseFiresThenRearmsAfterSkip(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "Control", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "PopupOKActivationPulse", Addr: "DB300.DBX1.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\nif Control == true then\n  once pulse PopupOKActivationPulse, 2;\nend;\n"
	prog, ds := simlang.Compile("once-pulse.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	r := NewRunner(prog, img)

	_ = img.Set("Control", false)
	if _, err := r.Step(); err != nil {
		t.Fatalf("step t0 failed: %v", err)
	}
	v, _ := img.Get("PopupOKActivationPulse")
	if b, _ := v.(bool); b {
		t.Fatalf("t0: expected false")
	}

	_ = img.Set("Control", true)
	t1, err := r.Step()
	if err != nil {
		t.Fatalf("step t1 failed: %v", err)
	}
	if !hasPulseEvent(t1, "pulse PopupOKActivationPulse fire n=2") {
		t.Fatalf("expected pulse fire event at t1, got %+v", t1)
	}
	v, _ = img.Get("PopupOKActivationPulse")
	if b, _ := v.(bool); !b {
		t.Fatalf("t1: expected true")
	}

	t2, err := r.Step()
	if err != nil {
		t.Fatalf("step t2 failed: %v", err)
	}
	if !hasPulseEvent(t2, "once pulse PopupOKActivationPulse skipped") {
		t.Fatalf("expected once-suppressed event at t2, got %+v", t2)
	}
	v, _ = img.Get("PopupOKActivationPulse")
	if b, _ := v.(bool); !b {
		t.Fatalf("t2: expected true")
	}

	t3, err := r.Step()
	if err != nil {
		t.Fatalf("step t3 failed: %v", err)
	}
	if !hasPulseEvent(t3, "once pulse PopupOKActivationPulse skipped") {
		t.Fatalf("expected once-suppressed event at t3, got %+v", t3)
	}
	v, _ = img.Get("PopupOKActivationPulse")
	if b, _ := v.(bool); b {
		t.Fatalf("t3: expected false after width expiry")
	}

	_ = img.Set("Control", false)
	if _, err := r.Step(); err != nil {
		t.Fatalf("step t4 failed: %v", err)
	}

	_ = img.Set("Control", true)
	t5, err := r.Step()
	if err != nil {
		t.Fatalf("step t5 failed: %v", err)
	}
	if !hasPulseEvent(t5, "pulse PopupOKActivationPulse fire n=2") {
		t.Fatalf("expected second pulse fire event at t5, got %+v", t5)
	}
}

func TestOncePulseTrueFiveTicksFiresOnceAndDropsFalse(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "Control", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "Lamp", Addr: "DB300.DBX1.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\nif Control == true then\n  once pulse Lamp, 2;\nend;\n"
	prog, ds := simlang.Compile("once-five.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	r := NewRunner(prog, img)
	_ = img.Set("Control", true)

	got := make([]bool, 0, 5)
	fireCount := 0
	for i := 0; i < 5; i++ {
		changes, err := r.Step()
		if err != nil {
			t.Fatalf("step %d failed: %v", i+1, err)
		}
		fireCount += countPulseEvent(changes, "pulse Lamp fire n=2")
		v, _ := img.Get("Lamp")
		got = append(got, v.(bool))
	}
	want := []bool{true, true, false, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value mismatch at step %d: got=%v want=%v full=%v", i+1, got[i], want[i], got)
		}
	}
	if fireCount != 1 {
		t.Fatalf("expected exactly one fire, got %d", fireCount)
	}
}

func TestOncePulseCancelsWhenConditionBecomesFalse(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "Control", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "Lamp", Addr: "DB300.DBX1.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\nif Control == true then\n  once pulse Lamp, 2;\nend;\n"
	prog, ds := simlang.Compile("once-cancel.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	r := NewRunner(prog, img)

	_ = img.Set("Control", true)
	if _, err := r.Step(); err != nil {
		t.Fatalf("step 1 failed: %v", err)
	}
	v, _ := img.Get("Lamp")
	if b, _ := v.(bool); !b {
		t.Fatalf("step 1: expected true")
	}

	_ = img.Set("Control", false)
	changes, err := r.Step()
	if err != nil {
		t.Fatalf("step 2 failed: %v", err)
	}
	if !hasBoolChange(changes, "Lamp", false) {
		t.Fatalf("expected false value change on cancel, got %+v", changes)
	}
	if !hasPulseEvent(changes, "pulse Lamp cancel (statement skipped)") {
		t.Fatalf("expected cancel event, got %+v", changes)
	}
	v, _ = img.Get("Lamp")
	if b, _ := v.(bool); b {
		t.Fatalf("step 2: expected false after cancel")
	}
}

func TestOncePulseDefaultWidthIsTwo(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "Control", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "Lamp", Addr: "DB300.DBX1.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\nif Control == true then\n  once pulse Lamp;\nend;\n"
	prog, ds := simlang.Compile("once-default.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	r := NewRunner(prog, img)
	_ = img.Set("Control", true)

	if _, err := r.Step(); err != nil {
		t.Fatalf("step 1 failed: %v", err)
	}
	v, _ := img.Get("Lamp")
	if b, _ := v.(bool); !b {
		t.Fatalf("step 1: expected true")
	}
	if _, err := r.Step(); err != nil {
		t.Fatalf("step 2 failed: %v", err)
	}
	v, _ = img.Get("Lamp")
	if b, _ := v.(bool); !b {
		t.Fatalf("step 2: expected true")
	}
	changes, err := r.Step()
	if err != nil {
		t.Fatalf("step 3 failed: %v", err)
	}
	if !hasBoolChange(changes, "Lamp", false) {
		t.Fatalf("expected false value change when pulse ends, got %+v", changes)
	}
	v, _ = img.Get("Lamp")
	if b, _ := v.(bool); b {
		t.Fatalf("expected false after pulse width ends")
	}
}

func TestOncePulseSitesLatchIndependentlyOnSameTag(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "ControlA", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "ControlB", Addr: "DB300.DBX1.1", Type: "BOOL"},
			{Name: "Lamp", Addr: "DB300.DBX1.2", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\nif ControlA == true then\n  once pulse Lamp, 2;\nend;\nif ControlB == true then\n  once pulse Lamp, 2;\nend;\n"
	prog, ds := simlang.Compile("two-sites.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	r := NewRunner(prog, img)

	_ = img.Set("ControlA", true)
	_ = img.Set("ControlB", true)
	changes, err := r.Step()
	if err != nil {
		t.Fatalf("step failed: %v", err)
	}
	if countPulseEvent(changes, "pulse Lamp fire n=2") != 1 {
		t.Fatalf("expected one fire event, got %+v", changes)
	}
	if countPulseEvent(changes, "once pulse Lamp skipped") != 0 {
		t.Fatalf("did not expect not-armed skip on first step, got %+v", changes)
	}
	v, _ := img.Get("Lamp")
	if b, _ := v.(bool); !b {
		t.Fatalf("expected Lamp true while pulse active")
	}

	changes, err = r.Step()
	if err != nil {
		t.Fatalf("second step failed: %v", err)
	}
	if countPulseEvent(changes, "once pulse Lamp skipped") < 1 {
		t.Fatalf("expected once skip event on latched second step, got %+v", changes)
	}
}

func TestPulseWithoutOnceCanRefireAfterIdle(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "Control", Addr: "DB300.DBX1.0", Type: "BOOL"},
			{Name: "Lamp", Addr: "DB300.DBX1.1", Type: "BOOL"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\nif Control == true then\n  pulse Lamp, 2;\nend;\n"
	prog, ds := simlang.Compile("pulse-plain.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	r := NewRunner(prog, img)

	_ = img.Set("Control", true)
	c1, err := r.Step()
	if err != nil {
		t.Fatalf("step1 failed: %v", err)
	}
	if countPulseEvent(c1, "pulse Lamp fire n=2") != 1 {
		t.Fatalf("expected first fire, got %+v", c1)
	}
	c2, err := r.Step()
	if err != nil {
		t.Fatalf("step2 failed: %v", err)
	}
	if countPulseEvent(c2, "pulse Lamp fire n=2") != 0 {
		t.Fatalf("did not expect retrigger while active, got %+v", c2)
	}
	c3, err := r.Step()
	if err != nil {
		t.Fatalf("step3 failed: %v", err)
	}
	if countPulseEvent(c3, "pulse Lamp fire n=2") != 1 {
		t.Fatalf("expected refire once idle, got %+v", c3)
	}
}

func hasPulseEvent(changes []Change, event string) bool {
	return countPulseEvent(changes, event) > 0
}

func countPulseEvent(changes []Change, event string) int {
	count := 0
	for _, c := range changes {
		if strings.Contains(c.Event, event) {
			count++
		}
	}
	return count
}

func hasBoolChange(changes []Change, name string, value bool) bool {
	for _, c := range changes {
		if c.Event != "" || c.Name != name {
			continue
		}
		b, ok := c.Value.(bool)
		if ok && b == value {
			return true
		}
	}
	return false
}

func TestStringAssignmentTruncatesToDeclaredLength(t *testing.T) {
	s := schema.Schema{
		Version: 1, DB: 300, Endian: "big", Size: schema.SizeSpec{Auto: true},
		Tags: []schema.Tag{
			{Name: "StatusText", Addr: "DB300.DBB20", Type: "STRING[4]"},
		},
	}
	_ = s.Normalize(&s.DB)
	src := "tick 100ms;\nStatusText := \"abcdef\";\n"
	prog, ds := simlang.Compile("a.sim", src, s)
	if len(ds) > 0 {
		t.Fatalf("compile failed: %s", ds[0].String())
	}
	img := NewImageFromSchema(s, prog)
	r := NewRunner(prog, img)
	if _, err := r.Step(); err != nil {
		t.Fatalf("step failed: %v", err)
	}
	v, _ := img.Get("StatusText")
	if got := v.(string); got != "abcd" {
		t.Fatalf("expected truncation to abcd, got %q", got)
	}
	if !img.ConsumeStringTruncated("StatusText") {
		t.Fatalf("expected truncation marker")
	}
}

func TestStringEncodeRawHeaderAndPayload(t *testing.T) {
	spec := decode.TypeSpec{Name: "STRING", StringLen: 5, SizeBytes: 7}
	payload, err := decode.EncodeRaw(spec, binary.BigEndian, "abc")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if len(payload) != 7 {
		t.Fatalf("unexpected payload size: %d", len(payload))
	}
	if payload[0] != 5 || payload[1] != 3 {
		t.Fatalf("unexpected string header: %v", payload[:2])
	}
	if string(payload[2:5]) != "abc" {
		t.Fatalf("unexpected payload content: %q", string(payload[2:5]))
	}
}
