package lsp

import (
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
)

func TestDiagToLSPMatchesCompileLineAndColumn(t *testing.T) {
	src := "tick 100ms;\nif OSServiceControl == true then\nAlarmMessage := \"Meu alarme\nend\nif OSServiceControl == false then\nAlarmMessage := \"\";\nend\n"
	diag := diagToLSP(src, simlang.Diag{Line: 3, Col: 19, Msg: "unterminated string", Hint: "close the string with \" before the end of the line"})
	if diag.Range.Start.Line != 2 {
		t.Fatalf("expected line 3 start, got %d", diag.Range.Start.Line+1)
	}
	if diag.Range.Start.Character <= 0 {
		t.Fatalf("expected character offset > 0, got %d", diag.Range.Start.Character)
	}
	if !strings.Contains(diag.Message, "unterminated string") {
		t.Fatalf("missing message: %q", diag.Message)
	}
}
