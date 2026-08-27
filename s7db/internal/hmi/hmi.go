package hmi

import (
	"strings"

	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

func ResolveUI(tag schema.Tag) schema.TagUI {
	ui := schema.TagUI{}
	if tag.UI != nil {
		ui = *tag.UI
	}
	typeName := strings.ToUpper(strings.TrimSpace(tag.Type))
	role := strings.ToLower(strings.TrimSpace(tag.Role))

	if ui.Widget == "" {
		switch {
		case role == "heartbeat":
			ui.Widget = "status"
		case typeName == "BOOL":
			ui.Widget = "button"
		case strings.HasPrefix(typeName, "STRING"):
			ui.Widget = "text"
		case typeName == "REAL":
			ui.Widget = "knob"
		case ui.Min != nil && ui.Max != nil:
			ui.Widget = "knob"
		default:
			ui.Widget = "number"
		}
	}
	if ui.Group == "" {
		if role == "heartbeat" {
			ui.Group = "System"
		} else {
			ui.Group = "General"
		}
	}
	if ui.Mode == "" && ui.Widget == "button" {
		ui.Mode = "toggle"
	}
	if ui.Widget == "knob" && typeName == "REAL" && ui.Step == nil {
		v := 0.1
		ui.Step = &v
	}
	return ui
}
