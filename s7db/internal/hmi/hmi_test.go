package hmi

import (
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

func TestResolveUI_REALDefaultsToKnobWithStep(t *testing.T) {
	tag := schema.Tag{
		Name: "Speed",
		Type: "REAL",
	}
	ui := ResolveUI(tag)
	if ui.Widget != "knob" {
		t.Fatalf("expected REAL widget knob, got %q", ui.Widget)
	}
	if ui.Step == nil || *ui.Step != 0.1 {
		t.Fatalf("expected REAL default step 0.1, got %#v", ui.Step)
	}
}

func TestResolveUI_ExplicitWidgetPreserved(t *testing.T) {
	step := 0.25
	tag := schema.Tag{
		Name: "Speed",
		Type: "REAL",
		UI: &schema.TagUI{
			Widget: "number",
			Step:   &step,
		},
	}
	ui := ResolveUI(tag)
	if ui.Widget != "number" {
		t.Fatalf("expected explicit widget to remain number, got %q", ui.Widget)
	}
	if ui.Step == nil || *ui.Step != 0.25 {
		t.Fatalf("expected explicit step to remain 0.25, got %#v", ui.Step)
	}
}
