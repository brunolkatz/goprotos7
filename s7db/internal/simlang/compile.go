package simlang

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
)

type ValueType string

const (
	TypeBool   ValueType = "BOOL"
	TypeInt    ValueType = "INT"
	TypeReal   ValueType = "REAL"
	TypeTime   ValueType = "TIME"
	TypeString ValueType = "STRING"
)

var stringTypeRE = regexp.MustCompile(`^STRING\[(\d+)\]$`)

type Symbol struct {
	Name      string
	Type      ValueType
	StringLen int
	IsTag     bool
	Role      string
	IsBuiltin bool
}

type Program struct {
	File       string
	Source     string
	AST        *AST
	Tick       time.Duration
	Symbols    map[string]Symbol
	UsedTags   map[string]struct{}
	Statements int
	Warnings   []Diag
}

func Compile(file, src string, sch schema.Schema) (*Program, []Diag) {
	ast, err := Parse(file, src)
	if err != nil {
		var pe *parseDiagError
		if errors.As(err, &pe) {
			return nil, []Diag{pe.Diag()}
		}
		return nil, []Diag{makeDiag(file, src, "parse error", err.Error(), "check token sequence and block endings", 1, 1)}
	}
	p := &Program{
		File:     file,
		Source:   src,
		AST:      ast,
		Symbols:  map[string]Symbol{},
		UsedTags: map[string]struct{}{},
	}
	diags := make([]Diag, 0)

	p.Symbols["tick"] = Symbol{Name: "tick", Type: TypeTime, IsBuiltin: true}
	tagNames := make([]string, 0, len(sch.Tags))
	db := sch.DB
	for _, t := range sch.Tags {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		a, _ := address.Parse(t.Addr, &db)
		sym, ok, warn := typeFromSchema(name, t.Type, a)
		if !ok {
			continue
		}
		sym.Role = strings.ToLower(strings.TrimSpace(t.Role))
		p.Symbols[name] = sym
		tagNames = append(tagNames, name)
		if warn != "" {
			p.Warnings = append(p.Warnings, makeDiag(file, src, "warning", warn, "use explicit STRING[n] in schema", 1, 1))
		}
	}

	td, err := parseDuration(ast.Tick.Duration)
	if err != nil {
		diags = append(diags, makeDiag(file, src, "compile error", fmt.Sprintf("invalid tick duration %q", ast.Tick.Duration), "use values like 100ms, 1s, T#5s", ast.Tick.Pos.Line, ast.Tick.Pos.Column))
	} else {
		p.Tick = td
	}
	for _, v := range ast.Vars {
		sym, ok, d := typeFromDecl(v)
		if !ok {
			if d.File == "" {
				d.File = file
				if d.Snippet == "" {
					lines := strings.Split(src, "\n")
					if d.Line > 0 && d.Line <= len(lines) {
						d.Snippet = lines[d.Line-1]
					}
				}
			}
			diags = append(diags, d)
			continue
		}
		if _, exists := p.Symbols[v.Name]; exists {
			diags = append(diags, makeDiag(file, src, "compile error", fmt.Sprintf("name %q already declared", v.Name), "rename variable or schema tag", v.Pos.Line, v.Pos.Column))
			continue
		}
		p.Symbols[v.Name] = Symbol{Name: v.Name, Type: sym.Type, StringLen: sym.StringLen}
		if v.Init != nil {
			t, n, d := inferExprType(file, src, p, v.Init, tagNames)
			diags = append(diags, d...)
			if len(d) == 0 && !assignable(p.Symbols[v.Name], t, n) {
				diags = append(diags, makeDiag(file, src, "type error", fmt.Sprintf("cannot assign %s to %s", renderType(t, n), renderType(sym.Type, sym.StringLen)), fmt.Sprintf("%s is %s", v.Name, renderType(sym.Type, sym.StringLen)), v.Pos.Line, v.Pos.Column))
			}
		}
	}
	for _, st := range ast.Stmts {
		diags = append(diags, checkStmt(file, src, p, st, tagNames)...)
		p.Statements++
	}
	if len(diags) > 0 {
		return nil, diags
	}
	return p, nil
}

func checkStmt(file, src string, p *Program, st *Stmt, names []string) []Diag {
	switch {
	case st.Assign != nil:
		return checkAssign(file, src, p, st.Assign, names)
	case st.If != nil:
		ds := []Diag{}
		t, _, d := inferExprType(file, src, p, st.If.Cond, names)
		ds = append(ds, d...)
		if len(d) == 0 && t != TypeBool {
			ds = append(ds, makeDiag(file, src, "type error", "if condition must be BOOL", "", st.If.Pos.Line, st.If.Pos.Column))
		}
		for _, s := range st.If.Then {
			ds = append(ds, checkStmt(file, src, p, s, names)...)
		}
		for _, s := range st.If.Else {
			ds = append(ds, checkStmt(file, src, p, s, names)...)
		}
		return ds
	case st.On != nil:
		ds := []Diag{}
		if st.On.Cond.Expr != nil {
			t, _, d := inferExprType(file, src, p, st.On.Cond.Expr, names)
			ds = append(ds, d...)
			if len(d) == 0 && t != TypeBool {
				ds = append(ds, makeDiag(file, src, "type error", "on condition must be BOOL", "", st.On.Pos.Line, st.On.Pos.Column))
			}
		}
		if st.On.Cond.Rising != nil || st.On.Cond.Falling != nil {
			n := deref(st.On.Cond.Rising)
			if n == "" {
				n = deref(st.On.Cond.Falling)
			}
			sym, ok := p.Symbols[n]
			if !ok {
				h := suggest(n, names)
				ds = append(ds, makeDiag(file, src, "compile error", fmt.Sprintf("unknown name %q", n), h, st.On.Pos.Line, st.On.Pos.Column))
			} else if sym.Type != TypeBool {
				ds = append(ds, makeDiag(file, src, "type error", fmt.Sprintf("edge trigger requires BOOL name, got %s", renderType(sym.Type, sym.StringLen)), "", st.On.Pos.Line, st.On.Pos.Column))
			}
			if sym.IsTag {
				p.UsedTags[sym.Name] = struct{}{}
			}
		}
		for _, s := range st.On.Body {
			ds = append(ds, checkStmt(file, src, p, s, names)...)
		}
		return ds
	case st.Pulse != nil:
		return checkPulse(file, src, p, st.Pulse, names)
	default:
		return nil
	}
}

func checkPulse(file, src string, p *Program, pulse *PulseStmt, names []string) []Diag {
	nameCol := pulseNameColumn(pulse)
	sym, ok := p.Symbols[pulse.Name]
	if !ok {
		return []Diag{makeDiag(file, src, "compile error", fmt.Sprintf("unknown name %q", pulse.Name), suggest(pulse.Name, names), pulse.Pos.Line, nameCol)}
	}
	if sym.Role == "heartbeat" {
		return []Diag{makeDiag(file, src, "type error", "cannot pulse heartbeat tag", fmt.Sprintf("%s is role:heartbeat", pulse.Name), pulse.Pos.Line, nameCol)}
	}
	if sym.Type != TypeBool {
		return []Diag{makeDiag(file, src, "type error", "pulse requires BOOL", fmt.Sprintf("%s is %s; pulse is a momentary BOOL", pulse.Name, renderType(sym.Type, sym.StringLen)), pulse.Pos.Line, nameCol)}
	}
	if pulse.Width != nil {
		if *pulse.Width < 1 {
			hint := "use pulse X, 2;"
			if pulse.Once != nil {
				hint = "use once pulse X, 2;"
			}
			return []Diag{makeDiag(file, src, "parse error", "pulse width must be an integer ≥ 1", hint, pulse.Pos.Line, pulseWidthColumn(pulse))}
		}
	}
	if sym.IsTag {
		p.UsedTags[sym.Name] = struct{}{}
	}
	return nil
}

func pulseNameColumn(p *PulseStmt) int {
	prefix := "pulse "
	if p.Once != nil {
		prefix = "once pulse "
	}
	return p.Pos.Column + len(prefix)
}

func pulseWidthColumn(p *PulseStmt) int {
	prefix := "pulse "
	if p.Once != nil {
		prefix = "once pulse "
	}
	return p.Pos.Column + len(prefix) + len(p.Name) + 1
}

func checkAssign(file, src string, p *Program, as *AssignStmt, names []string) []Diag {
	sym, ok := p.Symbols[as.Name]
	if !ok {
		return []Diag{makeDiag(file, src, "compile error", fmt.Sprintf("unknown name %q", as.Name), suggest(as.Name, names), as.Pos.Line, as.Pos.Column)}
	}
	if sym.IsBuiltin {
		return []Diag{makeDiag(file, src, "compile error", "cannot assign to builtin tick", "", as.Pos.Line, as.Pos.Column)}
	}
	t, n, ds := inferExprType(file, src, p, as.Expr, names)
	if len(ds) > 0 {
		return ds
	}
	if sym.IsTag {
		p.UsedTags[sym.Name] = struct{}{}
	}
	if !assignable(sym, t, n) {
		return []Diag{makeDiag(file, src, "type error", fmt.Sprintf("cannot assign %s to %s", renderType(t, n), renderType(sym.Type, sym.StringLen)), fmt.Sprintf("%s is %s", as.Name, renderType(sym.Type, sym.StringLen)), as.Pos.Line, as.Pos.Column)}
	}
	return nil
}

func inferExprType(file, src string, p *Program, ex *Expr, names []string) (ValueType, int, []Diag) {
	return inferOr(file, src, p, ex.Or, names)
}

func inferOr(file, src string, p *Program, ex *OrExpr, names []string) (ValueType, int, []Diag) {
	l, ln, d := inferAnd(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", 0, d
	}
	for _, r := range ex.Right {
		t, _, d := inferAnd(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", 0, d
		}
		if l != TypeBool || t != TypeBool {
			return "", 0, []Diag{makeDiag(file, src, "type error", "and/or require BOOL operands", "", r.Pos.Line, r.Pos.Column)}
		}
		l = TypeBool
	}
	return l, ln, nil
}

func inferAnd(file, src string, p *Program, ex *AndExpr, names []string) (ValueType, int, []Diag) {
	l, ln, d := inferCompare(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", 0, d
	}
	for _, r := range ex.Right {
		t, _, d := inferCompare(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", 0, d
		}
		if l != TypeBool || t != TypeBool {
			return "", 0, []Diag{makeDiag(file, src, "type error", "and/or require BOOL operands", "", r.Pos.Line, r.Pos.Column)}
		}
		l = TypeBool
	}
	return l, ln, nil
}

func inferCompare(file, src string, p *Program, ex *CompareExpr, names []string) (ValueType, int, []Diag) {
	l, ln, d := inferAdd(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", 0, d
	}
	for _, r := range ex.Right {
		t, tn, d := inferAdd(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", 0, d
		}
		if r.Op == "==" || r.Op == "!=" {
			if !(l == t || (isNumeric(l) && isNumeric(t)) || (l == TypeString && t == TypeString)) {
				return "", 0, []Diag{makeDiag(file, src, "type error", fmt.Sprintf("cannot compare %s and %s", renderType(l, ln), renderType(t, tn)), "", r.Pos.Line, r.Pos.Column)}
			}
		} else {
			if l == TypeString || t == TypeString {
				return "", 0, []Diag{makeDiag(file, src, "type error", "string ordering comparison is not supported", "use == or !=", r.Pos.Line, r.Pos.Column)}
			}
			if !((isNumeric(l) && isNumeric(t)) || (l == TypeTime && t == TypeTime)) {
				return "", 0, []Diag{makeDiag(file, src, "type error", fmt.Sprintf("operator %s requires numeric or TIME operands", r.Op), "", r.Pos.Line, r.Pos.Column)}
			}
		}
		l = TypeBool
		ln = 0
	}
	return l, ln, nil
}

func inferAdd(file, src string, p *Program, ex *AddExpr, names []string) (ValueType, int, []Diag) {
	l, ln, d := inferMul(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", 0, d
	}
	for _, r := range ex.Right {
		t, tn, d := inferMul(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", 0, d
		}
		if l == TypeString || t == TypeString {
			return "", 0, []Diag{makeDiag(file, src, "type error", "string concatenation not supported", "assign a full literal", r.Pos.Line, r.Pos.Column)}
		}
		if isNumeric(l) && isNumeric(t) {
			if l == TypeReal || t == TypeReal {
				l = TypeReal
			} else {
				l = TypeInt
			}
			ln = 0
			continue
		}
		if l == TypeTime && t == TypeTime && (r.Op == "+" || r.Op == "-") {
			l = TypeTime
			ln = 0
			continue
		}
		return "", 0, []Diag{makeDiag(file, src, "type error", fmt.Sprintf("invalid %s operands %s and %s", r.Op, renderType(l, ln), renderType(t, tn)), "", r.Pos.Line, r.Pos.Column)}
	}
	return l, ln, nil
}

func inferMul(file, src string, p *Program, ex *MulExpr, names []string) (ValueType, int, []Diag) {
	l, ln, d := inferUnary(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", 0, d
	}
	for _, r := range ex.Right {
		t, tn, d := inferUnary(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", 0, d
		}
		if l == TypeString || t == TypeString {
			return "", 0, []Diag{makeDiag(file, src, "type error", "string arithmetic not supported", "", r.Pos.Line, r.Pos.Column)}
		}
		if !(isNumeric(l) && isNumeric(t)) {
			return "", 0, []Diag{makeDiag(file, src, "type error", fmt.Sprintf("invalid %s operands %s and %s", r.Op, renderType(l, ln), renderType(t, tn)), "", r.Pos.Line, r.Pos.Column)}
		}
		if l == TypeReal || t == TypeReal {
			l = TypeReal
		} else {
			l = TypeInt
		}
		ln = 0
	}
	return l, ln, nil
}

func inferUnary(file, src string, p *Program, ex *UnaryExpr, names []string) (ValueType, int, []Diag) {
	t, n, d := inferPrimary(file, src, p, ex.Prim, names)
	if len(d) > 0 {
		return "", 0, d
	}
	if ex.Neg != nil && !(t == TypeInt || t == TypeReal || t == TypeTime) {
		return "", 0, []Diag{makeDiag(file, src, "type error", fmt.Sprintf("cannot negate %s", renderType(t, n)), "", ex.Pos.Line, ex.Pos.Column)}
	}
	return t, n, nil
}

func inferPrimary(file, src string, p *Program, ex *Primary, names []string) (ValueType, int, []Diag) {
	switch {
	case ex.BoolLit != nil:
		return TypeBool, 0, nil
	case ex.Duration != nil:
		if _, err := parseDuration(*ex.Duration); err != nil {
			return "", 0, []Diag{makeDiag(file, src, "type error", fmt.Sprintf("invalid TIME literal %q", *ex.Duration), "use T#100ms / T#5s / T#1m", ex.Pos.Line, ex.Pos.Column)}
		}
		return TypeTime, 0, nil
	case ex.StringLit != nil:
		s, err := ParseStringLiteral(*ex.StringLit)
		if err != nil {
			return "", 0, []Diag{makeDiag(file, src, "type error", err.Error(), "", ex.Pos.Line, ex.Pos.Column)}
		}
		return TypeString, len(s), nil
	case ex.Float != nil:
		return TypeReal, 0, nil
	case ex.Int != nil:
		return TypeInt, 0, nil
	case ex.Ident != nil:
		sym, ok := p.Symbols[*ex.Ident]
		if !ok {
			return "", 0, []Diag{makeDiag(file, src, "compile error", fmt.Sprintf("unknown name %q", *ex.Ident), suggest(*ex.Ident, names), ex.Pos.Line, ex.Pos.Column)}
		}
		if sym.IsTag {
			p.UsedTags[sym.Name] = struct{}{}
		}
		return sym.Type, sym.StringLen, nil
	case ex.SubExpr != nil:
		return inferExprType(file, src, p, ex.SubExpr, names)
	default:
		return "", 0, []Diag{makeDiag(file, src, "compile error", "invalid expression", "", ex.Pos.Line, ex.Pos.Column)}
	}
}

func ParseStringLiteral(raw string) (string, error) {
	s, err := strconv.Unquote(raw)
	if err != nil {
		return "", fmt.Errorf("invalid string literal %q", raw)
	}
	return s, nil
}

func parseDuration(raw string) (time.Duration, error) {
	s := strings.TrimSpace(strings.ToUpper(raw))
	s = strings.TrimPrefix(s, "T#")
	s = strings.ReplaceAll(s, "US", "µs")
	return time.ParseDuration(strings.ToLower(s))
}

func typeFromDecl(v *VarDecl) (Symbol, bool, Diag) {
	t := strings.ToUpper(strings.TrimSpace(v.Type))
	switch t {
	case "BOOL":
		return Symbol{Type: TypeBool}, true, Diag{}
	case "INT", "DINT", "UINT", "WORD", "DWORD":
		return Symbol{Type: TypeInt}, true, Diag{}
	case "REAL":
		return Symbol{Type: TypeReal}, true, Diag{}
	case "TIME":
		return Symbol{Type: TypeTime}, true, Diag{}
	case "STRING":
		if v.TypeLen == nil {
			return Symbol{}, false, makeDiag("", "", "type error", "STRING requires a max length", "use STRING[20] (1..254)", v.Pos.Line, v.Pos.Column+len("var ")+len(v.Name)+len(" : "))
		}
		n, _ := strconv.Atoi(*v.TypeLen)
		if n < 1 || n > 254 {
			return Symbol{}, false, makeDiag("", "", "type error", fmt.Sprintf("invalid STRING length %d", n), "use STRING[1..254]", v.Pos.Line, v.Pos.Column)
		}
		return Symbol{Type: TypeString, StringLen: n}, true, Diag{}
	default:
		return Symbol{}, false, makeDiag("", "", "type error", fmt.Sprintf("unknown var type %q", v.Type), "supported: BOOL INT DINT UINT WORD DWORD REAL TIME STRING[n]", v.Pos.Line, v.Pos.Column)
	}
}

func typeFromSchema(name, raw string, addrValue address.Address) (Symbol, bool, string) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	switch t {
	case "BOOL":
		return Symbol{Name: name, Type: TypeBool, IsTag: true}, true, ""
	case "INT", "DINT", "UINT", "WORD", "DWORD", "BYTE", "SINT", "USINT", "UDINT":
		return Symbol{Name: name, Type: TypeInt, IsTag: true}, true, ""
	case "REAL":
		return Symbol{Name: name, Type: TypeReal, IsTag: true}, true, ""
	case "TIME":
		return Symbol{Name: name, Type: TypeTime, IsTag: true}, true, ""
	}
	if m := stringTypeRE.FindStringSubmatch(t); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 254 {
			return Symbol{}, false, ""
		}
		return Symbol{Name: name, Type: TypeString, StringLen: n, IsTag: true}, true, ""
	}
	if strings.HasPrefix(t, "STRING") || t == "WSTRING" {
		n := addrValue.Bit
		if n < 1 || n > 254 {
			n = 254
			return Symbol{Name: name, Type: TypeString, StringLen: n, IsTag: true}, true, fmt.Sprintf("schema tag %s has STRING without explicit length; defaulting to STRING[254]", name)
		}
		return Symbol{Name: name, Type: TypeString, StringLen: n, IsTag: true}, true, ""
	}
	return Symbol{}, false, ""
}

func isNumeric(t ValueType) bool {
	return t == TypeInt || t == TypeReal
}

func assignable(dst Symbol, srcType ValueType, srcStringLen int) bool {
	if dst.Type == srcType {
		if dst.Type != TypeString {
			return true
		}
		return dst.StringLen > 0 && srcStringLen >= 0
	}
	return dst.Type == TypeReal && srcType == TypeInt
}

func renderType(t ValueType, n int) string {
	if t == TypeString && n > 0 {
		return fmt.Sprintf("STRING[%d]", n)
	}
	return string(t)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func suggest(name string, names []string) string {
	best := ""
	bestDist := math.MaxInt
	for _, n := range names {
		d := levenshtein(strings.ToLower(name), strings.ToLower(n))
		if d < bestDist {
			bestDist = d
			best = n
		}
	}
	if best != "" && bestDist <= 3 {
		return fmt.Sprintf("did you mean %s?", best)
	}
	return ""
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
		dp[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		dp[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			dp[i][j] = min3(dp[i-1][j]+1, dp[i][j-1]+1, dp[i-1][j-1]+cost)
		}
	}
	return dp[len(a)][len(b)]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func ParseNumber(raw string) (any, error) {
	if strings.Contains(raw, ".") {
		return strconv.ParseFloat(raw, 64)
	}
	return strconv.ParseInt(raw, 10, 64)
}
