package simlang

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var lex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `//[^\n]*`},
	{Name: "Whitespace", Pattern: `[ \t\r\n]+`},
	{Name: "Duration", Pattern: `(?:T#)?[0-9]+(?:ns|us|µs|ms|s|m|h)`},
	{Name: "String", Pattern: `"([^"\\]|\\.)*"`},
	{Name: "Float", Pattern: `[0-9]+\.[0-9]+`},
	{Name: "Int", Pattern: `[0-9]+`},
	{Name: "Operator", Pattern: `:=|==|!=|>=|<=|[+,\-*/><():;\[\]]`},
	{Name: "Ident", Pattern: `[A-Za-z_][A-Za-z0-9_]*`},
})

var parser = participle.MustBuild[AST](
	participle.Lexer(lex),
	participle.Elide("Whitespace", "Comment"),
	participle.UseLookahead(2),
	participle.CaseInsensitive("true", "false", "if", "then", "else", "end", "on", "do", "var", "tick", "pulse", "once", "and", "or", "rising", "falling"),
)

type parseDiagError struct {
	diag Diag
}

func (e *parseDiagError) Error() string {
	return e.diag.Msg
}

func (e *parseDiagError) Diag() Diag {
	return e.diag
}

var invalidInputRE = regexp.MustCompile(`invalid input text "([^"]*)"`)
var oncePulsePrefixRE = regexp.MustCompile(`^once[ \t]+pulse\b`)
var pulseOncePrefixRE = regexp.MustCompile(`^pulse[ \t]+once\b`)
var oncePrefixRE = regexp.MustCompile(`^once(?:[ \t;]|$)`)

func Parse(file, src string) (*AST, error) {
	if d, ok := detectUnterminatedString(file, src); ok {
		return nil, &parseDiagError{diag: d}
	}
	if d, ok := detectOnceMisuse(file, src); ok {
		return nil, &parseDiagError{diag: d}
	}
	if d, ok := detectSemicolonMisuse(file, src); ok {
		return nil, &parseDiagError{diag: d}
	}
	ast, err := parser.ParseString(file, src)
	if err != nil {
		return nil, &parseDiagError{diag: diagFromParseError(file, src, err)}
	}
	return ast, nil
}

func diagFromParseError(file, src string, err error) Diag {
	msg := err.Error()
	line, col := 1, 1

	var pErr participle.Error
	if errors.As(err, &pErr) {
		if p := pErr.Position(); p.Line > 0 {
			line = p.Line
			col = p.Column
		}
		msg = pErr.Message()
	}
	msg = stripPathPrefix(msg)

	hint := "check token sequence and block endings"
	if m := invalidInputRE.FindStringSubmatch(strings.ToLower(msg)); m != nil {
		msg = fmt.Sprintf("unexpected characters %q", m[1])
	}
	if strings.Contains(msg, "expected") && strings.Contains(msg, "\";\"") {
		msg = semicolonExpectationMessage(src, line, col)
		hint = `statements end with ";"`
	}
	if strings.Contains(msg, `unexpected token ";"`) {
		if atKeywordTerminator(src, line, col) {
			hint = `";" ends a statement; do not put it between if and then`
		}
		msg = `unexpected ";"`
	}
	if line <= 0 {
		line = 1
	}
	if col <= 0 {
		col = 1
	}
	return makeDiag(file, src, "parse error", msg, hint, line, col)
}

func stripPathPrefix(msg string) string {
	i := strings.Index(msg, ": ")
	if i <= 0 {
		return msg
	}
	prefix := msg[:i]
	if strings.Count(prefix, ":") >= 2 {
		return strings.TrimSpace(msg[i+2:])
	}
	return msg
}

func detectUnterminatedString(file, src string) (Diag, bool) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		inString := false
		escaped := false
		startCol := 1
		col := 0
		runes := []rune(line)
		for idx := 0; idx < len(runes); idx++ {
			ch := runes[idx]
			col++
			if !inString && ch == '/' && idx+1 < len(runes) && runes[idx+1] == '/' {
				break
			}
			if ch == '"' && !escaped {
				if inString {
					inString = false
				} else {
					inString = true
					startCol = col
				}
				continue
			}
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
				}
			}
		}
		if inString {
			return makeDiag(file, src, "parse error", "unterminated string", `close the string with " before the end of the line`, i+1, startCol), true
		}
	}
	return Diag{}, false
}

func detectOnceMisuse(file, src string) (Diag, bool) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		code := stripLineComment(line)
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if oncePrefixRE.MatchString(lower) && !oncePulsePrefixRE.MatchString(lower) {
			col := colFromTrimmed(line, trimmed, 1)
			return makeDiag(file, src, "parse error", `"once" can only prefix pulse`, "write  once pulse Tag, 2;", i+1, col), true
		}
		if pulseOncePrefixRE.MatchString(lower) {
			oncePos := strings.Index(lower, "once")
			col := colFromTrimmed(line, trimmed, oncePos+1)
			return makeDiag(file, src, "parse error", `"once" can only prefix pulse`, "write  once pulse Tag, 2;", i+1, col), true
		}
	}
	return Diag{}, false
}

func detectSemicolonMisuse(file, src string) (Diag, bool) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		code := stripLineComment(line)
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if semPos, ok := tokenWithSemicolon(lower, "then"); ok {
			col := colFromTrimmed(line, trimmed, semPos+1)
			return makeDiag(file, src, "parse error", `unexpected ";"`, `";" ends a statement; do not put it between if and then`, i+1, col), true
		}
		if semPos, ok := tokenWithSemicolon(lower, "do"); ok {
			col := colFromTrimmed(line, trimmed, semPos+1)
			return makeDiag(file, src, "parse error", `unexpected ";"`, `";" ends a statement; do not put it between if and then`, i+1, col), true
		}
		if semPos, ok := tokenWithSemicolon(lower, "else"); ok {
			col := colFromTrimmed(line, trimmed, semPos+1)
			return makeDiag(file, src, "parse error", `unexpected ";"`, `";" ends a statement; do not put it between if and then`, i+1, col), true
		}
		if strings.HasPrefix(lower, "if ") && strings.Contains(lower, " then") {
			if semPos := strings.Index(lower, ";"); semPos >= 0 && semPos < strings.Index(lower, " then") {
				col := colFromTrimmed(line, trimmed, semPos+1)
				return makeDiag(file, src, "parse error", `unexpected ";"`, `";" ends a statement; do not put it between if and then`, i+1, col), true
			}
			continue
		}
		if strings.HasPrefix(lower, "on ") && strings.Contains(lower, " do") {
			if semPos := strings.Index(lower, ";"); semPos >= 0 && semPos < strings.Index(lower, " do") {
				col := colFromTrimmed(line, trimmed, semPos+1)
				return makeDiag(file, src, "parse error", `unexpected ";"`, `";" ends a statement; do not put it between if and then`, i+1, col), true
			}
			continue
		}
		if strings.HasPrefix(lower, "end") {
			if strings.HasPrefix(lower, "end;;") {
				col := colFromTrimmed(line, trimmed, 5)
				return makeDiag(file, src, "parse error", `unexpected ";"`, "", i+1, col), true
			}
			continue
		}
		needSemicolon := strings.HasPrefix(lower, "tick ") || strings.HasPrefix(lower, "var ") || strings.HasPrefix(lower, "pulse ") || strings.HasPrefix(lower, "once pulse ") || strings.Contains(lower, ":=")
		if !needSemicolon {
			continue
		}
		if strings.HasSuffix(trimmed, ";;") {
			col := colFromTrimmed(line, trimmed, len([]rune(trimmed)))
			return makeDiag(file, src, "parse error", `unexpected ";"`, "", i+1, col), true
		}
		if !strings.HasSuffix(trimmed, ";") {
			col := len([]rune(strings.TrimRight(line, " \t"))) + 1
			msg := `expected ";" after assignment`
			switch {
			case strings.HasPrefix(lower, "tick "):
				msg = `expected ";" after tick`
			case strings.HasPrefix(lower, "var "):
				msg = `expected ";" after var declaration`
			case strings.HasPrefix(lower, "pulse "), strings.HasPrefix(lower, "once pulse "):
				msg = `expected ";" after pulse`
			}
			return makeDiag(file, src, "parse error", msg, `statements end with ";"`, i+1, max(1, col)), true
		}
	}
	return Diag{}, false
}

func stripLineComment(line string) string {
	inString := false
	escaped := false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '"' && !escaped {
			inString = !inString
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
			}
			continue
		}
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			return string(runes[:i])
		}
	}
	return line
}

func tokenWithSemicolon(line, token string) (int, bool) {
	idx := strings.Index(line, token)
	if idx < 0 {
		return -1, false
	}
	pos := idx + len(token)
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	if pos < len(line) && line[pos] == ';' {
		return pos, true
	}
	return -1, false
}

func colFromTrimmed(original, trimmed string, pos int) int {
	lead := len([]rune(original)) - len([]rune(strings.TrimLeft(original, " \t")))
	return lead + pos
}

func semicolonExpectationMessage(src string, line, col int) string {
	lines := strings.Split(src, "\n")
	if line <= 0 || line > len(lines) {
		return `expected ";"`
	}
	text := strings.TrimSpace(stripLineComment(lines[line-1]))
	switch {
	case strings.HasPrefix(strings.ToLower(text), "tick "):
		return `expected ";" after tick`
	case strings.HasPrefix(strings.ToLower(text), "var "):
		return `expected ";" after var declaration`
	case strings.HasPrefix(strings.ToLower(text), "pulse "), strings.HasPrefix(strings.ToLower(text), "once pulse "):
		return `expected ";" after pulse`
	default:
		return `expected ";"`
	}
}

func atKeywordTerminator(src string, line, col int) bool {
	lines := strings.Split(src, "\n")
	if line <= 0 || line > len(lines) {
		return false
	}
	r := []rune(lines[line-1])
	idx := col - 1
	if idx < 0 || idx >= len(r) || r[idx] != ';' {
		return false
	}
	prefix := strings.ToLower(string(r[:idx]))
	return strings.Contains(prefix, "if ") || strings.Contains(prefix, "on ") || strings.Contains(prefix, "then") || strings.Contains(prefix, "do") || strings.Contains(prefix, "else")
}

func byteOffsetToLineCol(src string, offset int) (int, int) {
	if offset <= 0 {
		return 1, 1
	}
	if offset > len(src) {
		offset = len(src)
	}
	line := 1
	col := 1
	for i, r := range src {
		if i >= offset {
			break
		}
		if r == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}

func parseLineColFromMessage(msg string) (int, int, bool) {
	parts := strings.Split(msg, ":")
	if len(parts) < 3 {
		return 0, 0, false
	}
	for i := len(parts) - 2; i >= 0; i-- {
		l, lErr := strconv.Atoi(strings.TrimSpace(parts[i]))
		c, cErr := strconv.Atoi(strings.TrimSpace(parts[i+1]))
		if lErr == nil && cErr == nil {
			return l, c, true
		}
	}
	return 0, 0, false
}
