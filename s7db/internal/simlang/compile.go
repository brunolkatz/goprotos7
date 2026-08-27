package simlang

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

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

type Symbol struct {
	Name      string
	Type      ValueType
	IsTag     bool
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
}

func Compile(file, src string, sch schema.Schema) (*Program, []Diag) {
	ast, err := Parse(file, src)
	if err != nil {
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
	for _, t := range sch.Tags {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		typ, ok := typeFromSchema(t.Type)
		if !ok {
			continue
		}
		p.Symbols[name] = Symbol{Name: name, Type: typ, IsTag: true}
		tagNames = append(tagNames, name)
	}

	td, err := parseDuration(ast.Tick.Duration)
	if err != nil {
		diags = append(diags, makeDiag(file, src, "compile error", fmt.Sprintf("invalid tick duration %q", ast.Tick.Duration), "use values like 100ms, 1s, T#5s", ast.Tick.Pos.Line, ast.Tick.Pos.Column))
	} else {
		p.Tick = td
	}
	for _, v := range ast.Vars {
		vType, ok := typeFromDecl(v.Type)
		if !ok {
			diags = append(diags, makeDiag(file, src, "type error", fmt.Sprintf("unknown var type %q", v.Type), "supported: BOOL INT DINT UINT WORD DWORD REAL TIME", v.Pos.Line, v.Pos.Column))
			continue
		}
		if _, exists := p.Symbols[v.Name]; exists {
			diags = append(diags, makeDiag(file, src, "compile error", fmt.Sprintf("name %q already declared", v.Name), "rename variable or schema tag", v.Pos.Line, v.Pos.Column))
			continue
		}
		p.Symbols[v.Name] = Symbol{Name: v.Name, Type: vType}
		if v.Init != nil {
			t, d := inferExprType(file, src, p, v.Init, tagNames)
			diags = append(diags, d...)
			if len(d) == 0 && !assignable(vType, t) {
				diags = append(diags, makeDiag(file, src, "type error", fmt.Sprintf("cannot assign %s to %s", t, vType), "", v.Pos.Line, v.Pos.Column))
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
		t, d := inferExprType(file, src, p, st.If.Cond, names)
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
			t, d := inferExprType(file, src, p, st.On.Cond.Expr, names)
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
				ds = append(ds, makeDiag(file, src, "type error", fmt.Sprintf("edge trigger requires BOOL name, got %s", sym.Type), "", st.On.Pos.Line, st.On.Pos.Column))
			}
			if sym.IsTag {
				p.UsedTags[sym.Name] = struct{}{}
			}
		}
		for _, s := range st.On.Body {
			ds = append(ds, checkStmt(file, src, p, s, names)...)
		}
		return ds
	default:
		return nil
	}
}

func checkAssign(file, src string, p *Program, as *AssignStmt, names []string) []Diag {
	sym, ok := p.Symbols[as.Name]
	if !ok {
		return []Diag{makeDiag(file, src, "compile error", fmt.Sprintf("unknown name %q", as.Name), suggest(as.Name, names), as.Pos.Line, as.Pos.Column)}
	}
	if sym.IsBuiltin {
		return []Diag{makeDiag(file, src, "compile error", "cannot assign to builtin tick", "", as.Pos.Line, as.Pos.Column)}
	}
	t, ds := inferExprType(file, src, p, as.Expr, names)
	if len(ds) > 0 {
		return ds
	}
	if sym.IsTag {
		p.UsedTags[sym.Name] = struct{}{}
		if sym.Type == TypeString {
			return []Diag{makeDiag(file, src, "type error", fmt.Sprintf("STRING writes not supported in v1 (%s)", as.Name), "remove assignment or switch to non-STRING target", as.Pos.Line, as.Pos.Column)}
		}
	}
	if !assignable(sym.Type, t) {
		return []Diag{makeDiag(file, src, "type error", fmt.Sprintf("cannot assign %s to %s", t, sym.Type), fmt.Sprintf("%s is %s", as.Name, sym.Type), as.Pos.Line, as.Pos.Column)}
	}
	return nil
}

func inferExprType(file, src string, p *Program, ex *Expr, names []string) (ValueType, []Diag) {
	return inferOr(file, src, p, ex.Or, names)
}

func inferOr(file, src string, p *Program, ex *OrExpr, names []string) (ValueType, []Diag) {
	l, d := inferAnd(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", d
	}
	for _, r := range ex.Right {
		t, d := inferAnd(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", d
		}
		if l != TypeBool || t != TypeBool {
			return "", []Diag{makeDiag(file, src, "type error", "and/or require BOOL operands", "", r.Pos.Line, r.Pos.Column)}
		}
		l = TypeBool
	}
	return l, nil
}

func inferAnd(file, src string, p *Program, ex *AndExpr, names []string) (ValueType, []Diag) {
	l, d := inferCompare(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", d
	}
	for _, r := range ex.Right {
		t, d := inferCompare(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", d
		}
		if l != TypeBool || t != TypeBool {
			return "", []Diag{makeDiag(file, src, "type error", "and/or require BOOL operands", "", r.Pos.Line, r.Pos.Column)}
		}
		l = TypeBool
	}
	return l, nil
}

func inferCompare(file, src string, p *Program, ex *CompareExpr, names []string) (ValueType, []Diag) {
	l, d := inferAdd(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", d
	}
	for _, r := range ex.Right {
		t, d := inferAdd(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", d
		}
		if r.Op == "==" || r.Op == "!=" {
			if !(l == t || (isNumeric(l) && isNumeric(t))) {
				return "", []Diag{makeDiag(file, src, "type error", fmt.Sprintf("cannot compare %s and %s", l, t), "", r.Pos.Line, r.Pos.Column)}
			}
		} else {
			if !((isNumeric(l) && isNumeric(t)) || (l == TypeTime && t == TypeTime)) {
				return "", []Diag{makeDiag(file, src, "type error", fmt.Sprintf("operator %s requires numeric or TIME operands", r.Op), "", r.Pos.Line, r.Pos.Column)}
			}
		}
		l = TypeBool
	}
	return l, nil
}

func inferAdd(file, src string, p *Program, ex *AddExpr, names []string) (ValueType, []Diag) {
	l, d := inferMul(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", d
	}
	for _, r := range ex.Right {
		t, d := inferMul(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", d
		}
		if isNumeric(l) && isNumeric(t) {
			if l == TypeReal || t == TypeReal {
				l = TypeReal
			} else {
				l = TypeInt
			}
			continue
		}
		if l == TypeTime && t == TypeTime && (r.Op == "+" || r.Op == "-") {
			l = TypeTime
			continue
		}
		return "", []Diag{makeDiag(file, src, "type error", fmt.Sprintf("invalid %s operands %s and %s", r.Op, l, t), "", r.Pos.Line, r.Pos.Column)}
	}
	return l, nil
}

func inferMul(file, src string, p *Program, ex *MulExpr, names []string) (ValueType, []Diag) {
	l, d := inferUnary(file, src, p, ex.Left, names)
	if len(d) > 0 {
		return "", d
	}
	for _, r := range ex.Right {
		t, d := inferUnary(file, src, p, r.Rhs, names)
		if len(d) > 0 {
			return "", d
		}
		if !(isNumeric(l) && isNumeric(t)) {
			return "", []Diag{makeDiag(file, src, "type error", fmt.Sprintf("invalid %s operands %s and %s", r.Op, l, t), "", r.Pos.Line, r.Pos.Column)}
		}
		if l == TypeReal || t == TypeReal {
			l = TypeReal
		} else {
			l = TypeInt
		}
	}
	return l, nil
}

func inferUnary(file, src string, p *Program, ex *UnaryExpr, names []string) (ValueType, []Diag) {
	t, d := inferPrimary(file, src, p, ex.Prim, names)
	if len(d) > 0 {
		return "", d
	}
	if ex.Neg != nil && !(t == TypeInt || t == TypeReal || t == TypeTime) {
		return "", []Diag{makeDiag(file, src, "type error", fmt.Sprintf("cannot negate %s", t), "", ex.Pos.Line, ex.Pos.Column)}
	}
	return t, nil
}

func inferPrimary(file, src string, p *Program, ex *Primary, names []string) (ValueType, []Diag) {
	switch {
	case ex.BoolLit != nil:
		return TypeBool, nil
	case ex.Duration != nil:
		if _, err := parseDuration(*ex.Duration); err != nil {
			return "", []Diag{makeDiag(file, src, "type error", fmt.Sprintf("invalid TIME literal %q", *ex.Duration), "use T#100ms / T#5s / T#1m", ex.Pos.Line, ex.Pos.Column)}
		}
		return TypeTime, nil
	case ex.Float != nil:
		return TypeReal, nil
	case ex.Int != nil:
		return TypeInt, nil
	case ex.Ident != nil:
		sym, ok := p.Symbols[*ex.Ident]
		if !ok {
			return "", []Diag{makeDiag(file, src, "compile error", fmt.Sprintf("unknown name %q", *ex.Ident), suggest(*ex.Ident, names), ex.Pos.Line, ex.Pos.Column)}
		}
		if sym.IsTag {
			p.UsedTags[sym.Name] = struct{}{}
		}
		return sym.Type, nil
	case ex.SubExpr != nil:
		return inferExprType(file, src, p, ex.SubExpr, names)
	default:
		return "", []Diag{makeDiag(file, src, "compile error", "invalid expression", "", ex.Pos.Line, ex.Pos.Column)}
	}
}

func parseDuration(raw string) (time.Duration, error) {
	s := strings.TrimSpace(strings.ToUpper(raw))
	s = strings.TrimPrefix(s, "T#")
	s = strings.ReplaceAll(s, "US", "µs")
	return time.ParseDuration(strings.ToLower(s))
}

func typeFromDecl(raw string) (ValueType, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "BOOL":
		return TypeBool, true
	case "INT", "DINT", "UINT", "WORD", "DWORD":
		return TypeInt, true
	case "REAL":
		return TypeReal, true
	case "TIME":
		return TypeTime, true
	default:
		return "", false
	}
}

func typeFromSchema(raw string) (ValueType, bool) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	switch t {
	case "BOOL":
		return TypeBool, true
	case "INT", "DINT", "UINT", "WORD", "DWORD", "BYTE", "SINT", "USINT", "UDINT":
		return TypeInt, true
	case "REAL":
		return TypeReal, true
	case "TIME":
		return TypeTime, true
	}
	if strings.HasPrefix(t, "STRING") || t == "WSTRING" {
		return TypeString, true
	}
	return "", false
}

func isNumeric(t ValueType) bool {
	return t == TypeInt || t == TypeReal
}

func assignable(dst, src ValueType) bool {
	if dst == src {
		return true
	}
	return dst == TypeReal && src == TypeInt
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
