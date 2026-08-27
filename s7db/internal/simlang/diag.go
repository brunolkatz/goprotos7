package simlang

import (
	"fmt"
	"strings"
)

type Diag struct {
	File    string
	Msg     string
	Hint    string
	Snippet string
	Kind    string
	Line    int
	Col     int
	Cycle   int
}

func (d Diag) String() string {
	var b strings.Builder
	kind := d.Kind
	if kind == "" {
		kind = "error"
	}
	if d.Cycle > 0 {
		fmt.Fprintf(&b, "%s:%d:%d: %s (cycle=%d): %s\n", d.File, d.Line, d.Col, kind, d.Cycle, d.Msg)
	} else {
		fmt.Fprintf(&b, "%s:%d:%d: %s: %s\n", d.File, d.Line, d.Col, kind, d.Msg)
	}
	if d.Snippet != "" {
		fmt.Fprintf(&b, "  %d | %s\n", d.Line, d.Snippet)
		pad := 4 + len(fmt.Sprintf("%d", d.Line)) + 3 + max(0, d.Col-1)
		fmt.Fprintf(&b, "%s^\n", strings.Repeat(" ", pad))
	}
	if d.Hint != "" {
		fmt.Fprintf(&b, "  hint: %s\n", d.Hint)
	}
	return strings.TrimRight(b.String(), "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func makeDiag(file, src, kind, msg, hint string, line, col int) Diag {
	snippet := ""
	lines := strings.Split(src, "\n")
	if line > 0 && line <= len(lines) {
		snippet = lines[line-1]
	}
	return Diag{
		File:    file,
		Kind:    kind,
		Msg:     msg,
		Hint:    hint,
		Snippet: snippet,
		Line:    line,
		Col:     col,
	}
}
