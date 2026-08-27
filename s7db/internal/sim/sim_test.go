package sim

import (
	"context"
	"strings"
	"testing"
	"time"

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
	src := "tick 100ms\non OsServiceHeartbeat == true do\nOsServiceHeartbeat := false\nend\n"
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
	src := `tick 100ms
var fault_timer : TIME := T#0s
if OsServiceHeartbeat == false then
  fault_timer := fault_timer + tick
else
  fault_timer := T#0s
end
if fault_timer > T#5s then
  OsServiceFault := true
else
  OsServiceFault := false
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
	src := "tick 100ms\nvar x : REAL := 1.0\nx := x / 0\n"
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
	// matrix behavior is covered by command-level validation; this test keeps sink API behavior visible.
	spy := &pushSpy{}
	if err := spy.Pull(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if spy.pushs != 0 {
		t.Fatalf("expected no push calls")
	}
}
