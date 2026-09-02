package sim

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
)

type Change struct {
	Name  string
	Value any
	Type  simlang.ValueType
	Event string
}

type Runner struct {
	Prog      *simlang.Program
	Img       *Image
	Tick      time.Duration
	Cycle     int
	lastBools map[string]bool
	pulses    map[string]*pulseSiteState
	tagOwner  map[string]string
}

type pulseSiteState struct {
	siteKey   string
	tag       string
	once      bool
	armed     bool
	remaining int
	active    bool
	executed  bool
	fired     bool
}

func NewRunner(prog *simlang.Program, img *Image) *Runner {
	return &Runner{
		Prog:      prog,
		Img:       img,
		Tick:      prog.Tick,
		lastBools: map[string]bool{},
		pulses:    map[string]*pulseSiteState{},
		tagOwner:  map[string]string{},
	}
}

func (r *Runner) Step() ([]Change, error) {
	r.Cycle++
	_ = r.Img.Set("tick", r.Tick)
	changes := make([]Change, 0)
	for _, ps := range r.pulses {
		ps.executed = false
		ps.fired = false
		if !ps.active {
			continue
		}
		ps.remaining--
		if ps.remaining <= 0 {
			if err := r.endPulse(ps); err != nil {
				return nil, err
			}
			changes = append(changes, Change{Name: ps.tag, Value: false, Type: simlang.TypeBool})
			changes = append(changes, Change{Name: ps.tag, Type: simlang.TypeBool, Event: fmt.Sprintf("pulse %s end", ps.tag)})
		}
	}
	for _, st := range r.Prog.AST.Stmts {
		cs, err := r.execStmt(st)
		if err != nil {
			return nil, err
		}
		changes = append(changes, cs...)
	}
	for _, ps := range r.pulses {
		if ps.active && !ps.executed {
			if err := r.endPulse(ps); err != nil {
				return nil, err
			}
			changes = append(changes, Change{Name: ps.tag, Value: false, Type: simlang.TypeBool})
			changes = append(changes, Change{Name: ps.tag, Type: simlang.TypeBool, Event: fmt.Sprintf("pulse %s cancel (statement skipped)", ps.tag)})
		}
		if ps.active && ps.executed && !ps.fired {
			changes = append(changes, Change{Name: ps.tag, Type: simlang.TypeBool, Event: fmt.Sprintf("pulse %s hold n=%d", ps.tag, ps.remaining)})
		}
		if ps.once && !ps.executed {
			ps.armed = true
		}
	}
	for name, typ := range r.Prog.Symbols {
		if typ.Type != simlang.TypeBool {
			continue
		}
		v, _ := r.Img.Get(name)
		b, _ := v.(bool)
		r.lastBools[name] = b
	}
	return mergeChanges(changes), nil
}

func mergeChanges(in []Change) []Change {
	m := map[string]Change{}
	order := make([]string, 0, len(in))
	events := make([]Change, 0)
	for _, c := range in {
		if c.Event != "" {
			events = append(events, c)
			continue
		}
		if _, ok := m[c.Name]; !ok {
			order = append(order, c.Name)
		}
		m[c.Name] = c
	}
	out := make([]Change, 0, len(order)+len(events))
	for _, name := range order {
		out = append(out, m[name])
	}
	out = append(out, events...)
	return out
}

func (r *Runner) execStmt(st *simlang.Stmt) ([]Change, error) {
	switch {
	case st.Assign != nil:
		return r.execAssign(st.Assign)
	case st.If != nil:
		v, err := r.eval(st.If.Cond)
		if err != nil {
			return nil, err
		}
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("runtime type error at %d:%d: if condition is not BOOL", st.If.Pos.Line, st.If.Pos.Column)
		}
		var list []*simlang.Stmt
		if b {
			list = st.If.Then
		} else {
			list = st.If.Else
		}
		var out []Change
		for _, s := range list {
			cs, err := r.execStmt(s)
			if err != nil {
				return nil, err
			}
			out = append(out, cs...)
		}
		return out, nil
	case st.On != nil:
		trigger, err := r.evalOn(st.On)
		if err != nil {
			return nil, err
		}
		if !trigger {
			return nil, nil
		}
		var out []Change
		for _, s := range st.On.Body {
			cs, err := r.execStmt(s)
			if err != nil {
				return nil, err
			}
			out = append(out, cs...)
		}
		return out, nil
	case st.Pulse != nil:
		return r.execPulse(st.Pulse)
	default:
		return nil, nil
	}
}

func (r *Runner) execPulse(p *simlang.PulseStmt) ([]Change, error) {
	siteKey := pulseSiteKey(r.Prog.File, p)
	ps, ok := r.pulses[siteKey]
	if !ok {
		ps = &pulseSiteState{
			siteKey: siteKey,
			tag:     p.Name,
			once:    p.Once != nil,
			armed:   true,
		}
		r.pulses[siteKey] = ps
	}
	ps.executed = true

	width := 2
	if p.Width != nil {
		width = *p.Width
	}
	if ps.once {
		if !ps.armed {
			return []Change{{Name: p.Name, Type: simlang.TypeBool, Event: fmt.Sprintf("once pulse %s skipped", p.Name)}}, nil
		}
		ps.armed = false
	}
	if ps.active {
		return nil, nil
	}
	if owner, busy := r.tagOwner[p.Name]; busy && owner != siteKey {
		return nil, nil
	}
	if err := r.Img.Set(p.Name, true); err != nil {
		return nil, fmt.Errorf("runtime pulse %s: %w", p.Name, err)
	}
	ps.active = true
	ps.remaining = width
	ps.fired = true
	r.tagOwner[p.Name] = siteKey
	return []Change{
		{Name: p.Name, Value: true, Type: simlang.TypeBool},
		{Name: p.Name, Type: simlang.TypeBool, Event: fmt.Sprintf("pulse %s fire n=%d site=%s", p.Name, width, siteKey)},
	}, nil
}

func (r *Runner) endPulse(ps *pulseSiteState) error {
	delete(r.tagOwner, ps.tag)
	ps.active = false
	ps.remaining = 0
	if err := r.Img.Set(ps.tag, false); err != nil {
		return fmt.Errorf("runtime pulse clear %s: %w", ps.tag, err)
	}
	return nil
}

func pulseSiteKey(file string, p *simlang.PulseStmt) string {
	return fmt.Sprintf("%s:%d:%d:%d", file, p.Pos.Offset, p.Pos.Line, p.Pos.Column)
}

func (r *Runner) evalOn(st *simlang.OnStmt) (bool, error) {
	if st.Cond.Expr != nil {
		v, err := r.eval(st.Cond.Expr)
		if err != nil {
			return false, err
		}
		b, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("runtime type error at %d:%d: on condition is not BOOL", st.Pos.Line, st.Pos.Column)
		}
		return b, nil
	}
	name := ""
	rising := false
	if st.Cond.Rising != nil {
		name = *st.Cond.Rising
		rising = true
	}
	if st.Cond.Falling != nil {
		name = *st.Cond.Falling
	}
	v, ok := r.Img.Get(name)
	if !ok {
		return false, fmt.Errorf("runtime unknown name %q", name)
	}
	cur, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("runtime edge trigger name %q is not BOOL", name)
	}
	prev := r.lastBools[name]
	if rising {
		return !prev && cur, nil
	}
	return prev && !cur, nil
}

func (r *Runner) execAssign(a *simlang.AssignStmt) ([]Change, error) {
	v, err := r.eval(a.Expr)
	if err != nil {
		return nil, err
	}
	old, _ := r.Img.Get(a.Name)
	if err := r.Img.Set(a.Name, v); err != nil {
		return nil, fmt.Errorf("runtime assign %s at %d:%d: %w", a.Name, a.Pos.Line, a.Pos.Column, err)
	}
	newV, _ := r.Img.Get(a.Name)
	if equal(old, newV) {
		return nil, nil
	}
	t, _ := r.Img.Type(a.Name)
	return []Change{{Name: a.Name, Value: newV, Type: t}}, nil
}

func (r *Runner) eval(ex *simlang.Expr) (any, error) {
	return r.evalOr(ex.Or)
}

func (r *Runner) evalOr(ex *simlang.OrExpr) (any, error) {
	lv, err := r.evalAnd(ex.Left)
	if err != nil {
		return nil, err
	}
	if len(ex.Right) == 0 {
		return lv, nil
	}
	lb := lv.(bool)
	for _, rr := range ex.Right {
		if lb {
			return true, nil
		}
		rv, err := r.evalAnd(rr.Rhs)
		if err != nil {
			return nil, err
		}
		lb = lb || rv.(bool)
	}
	return lb, nil
}

func (r *Runner) evalAnd(ex *simlang.AndExpr) (any, error) {
	lv, err := r.evalCompare(ex.Left)
	if err != nil {
		return nil, err
	}
	if len(ex.Right) == 0 {
		return lv, nil
	}
	lb := lv.(bool)
	for _, rr := range ex.Right {
		if !lb {
			return false, nil
		}
		rv, err := r.evalCompare(rr.Rhs)
		if err != nil {
			return nil, err
		}
		lb = lb && rv.(bool)
	}
	return lb, nil
}

func (r *Runner) evalCompare(ex *simlang.CompareExpr) (any, error) {
	lv, err := r.evalAdd(ex.Left)
	if err != nil {
		return nil, err
	}
	if len(ex.Right) == 0 {
		return lv, nil
	}
	for _, rr := range ex.Right {
		rv, err := r.evalAdd(rr.Rhs)
		if err != nil {
			return nil, err
		}
		ok, err := compare(lv, rv, rr.Op)
		if err != nil {
			return nil, fmt.Errorf("runtime compare at %d:%d: %w", rr.Pos.Line, rr.Pos.Column, err)
		}
		lv = ok
	}
	return lv, nil
}

func (r *Runner) evalAdd(ex *simlang.AddExpr) (any, error) {
	lv, err := r.evalMul(ex.Left)
	if err != nil {
		return nil, err
	}
	for _, rr := range ex.Right {
		rv, err := r.evalMul(rr.Rhs)
		if err != nil {
			return nil, err
		}
		lv, err = addsub(lv, rv, rr.Op)
		if err != nil {
			return nil, fmt.Errorf("runtime arithmetic at %d:%d: %w", rr.Pos.Line, rr.Pos.Column, err)
		}
	}
	return lv, nil
}

func (r *Runner) evalMul(ex *simlang.MulExpr) (any, error) {
	lv, err := r.evalUnary(ex.Left)
	if err != nil {
		return nil, err
	}
	for _, rr := range ex.Right {
		rv, err := r.evalUnary(rr.Rhs)
		if err != nil {
			return nil, err
		}
		lv, err = muldiv(lv, rv, rr.Op)
		if err != nil {
			return nil, fmt.Errorf("runtime arithmetic at %d:%d: %w", rr.Pos.Line, rr.Pos.Column, err)
		}
	}
	return lv, nil
}

func (r *Runner) evalUnary(ex *simlang.UnaryExpr) (any, error) {
	v, err := r.evalPrimary(ex.Prim)
	if err != nil {
		return nil, err
	}
	if ex.Neg == nil {
		return v, nil
	}
	switch x := v.(type) {
	case int64:
		return -x, nil
	case float64:
		return -x, nil
	case time.Duration:
		return -x, nil
	default:
		return nil, fmt.Errorf("runtime negate unsupported type %T", v)
	}
}

func (r *Runner) evalPrimary(p *simlang.Primary) (any, error) {
	switch {
	case p.BoolLit != nil:
		return stringsEqual(*p.BoolLit, "true"), nil
	case p.Int != nil:
		return simlang.ParseNumber(*p.Int)
	case p.Float != nil:
		return simlang.ParseNumber(*p.Float)
	case p.Duration != nil:
		return parseDuration(*p.Duration)
	case p.StringLit != nil:
		return simlang.ParseStringLiteral(*p.StringLit)
	case p.Ident != nil:
		v, ok := r.Img.Get(*p.Ident)
		if !ok {
			return nil, fmt.Errorf("runtime unknown name %q", *p.Ident)
		}
		return v, nil
	case p.SubExpr != nil:
		return r.eval(p.SubExpr)
	default:
		return nil, fmt.Errorf("runtime invalid primary expression")
	}
}

type Sink interface {
	Pull(ctx context.Context, names []string) error
	Push(ctx context.Context, changed []Change) error
}

type MemorySink struct{}

func (m MemorySink) Pull(context.Context, []string) error { return nil }
func (m MemorySink) Push(context.Context, []Change) error { return nil }

func stringsEqual(a, b string) bool { return strings.EqualFold(a, b) }
