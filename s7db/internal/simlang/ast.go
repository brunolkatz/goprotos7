package simlang

import "github.com/alecthomas/participle/v2/lexer"

type AST struct {
	Pos   lexer.Position `parser:""`
	Tick  *TickStmt      `@@`
	Vars  []*VarDecl     `@@*`
	Stmts []*Stmt        `@@*`
}

type TickStmt struct {
	Pos      lexer.Position `parser:""`
	Duration string         `"tick" @Duration ";"`
}

type VarDecl struct {
	Pos     lexer.Position `parser:""`
	Name    string         `"var" @Ident`
	Type    string         `":" @Ident`
	TypeLen *string        `( "[" @Int "]" )?`
	Init    *Expr          `( ":=" @@ )? ";"`
}

type Stmt struct {
	Pos    lexer.Position `parser:""`
	Assign *AssignStmt    `  @@`
	If     *IfStmt        `| @@`
	On     *OnStmt        `| @@`
	Pulse  *PulseStmt     `| @@`
}

type AssignStmt struct {
	Pos  lexer.Position `parser:""`
	Name string         `@Ident`
	Expr *Expr          `":=" @@ ";"`
}

type PulseStmt struct {
	Pos   lexer.Position `parser:""`
	Name  string         `"pulse" @Ident`
	Width *int           `( "," @Int )? ";"`
}

type IfStmt struct {
	Pos  lexer.Position `parser:""`
	Cond *Expr          `"if" @@ "then"`
	Then []*Stmt        `@@*`
	Else []*Stmt        `( "else" @@* )? "end" ";"?`
}

type OnStmt struct {
	Pos  lexer.Position `parser:""`
	Cond *OnCond        `"on" @@ "do"`
	Body []*Stmt        `@@* "end" ";"?`
}

type OnCond struct {
	Pos     lexer.Position `parser:""`
	Rising  *string        `  ( "rising" @Ident`
	Falling *string        `  | "falling" @Ident`
	Expr    *Expr          `  | @@ )`
}

type Expr struct {
	Pos lexer.Position `parser:""`
	Or  *OrExpr        `@@`
}

type OrExpr struct {
	Pos   lexer.Position `parser:""`
	Left  *AndExpr       `@@`
	Right []*OrRight     `( @@ )*`
}

type OrRight struct {
	Pos lexer.Position `parser:""`
	Op  string         `@( "or" )`
	Rhs *AndExpr       `@@`
}

type AndExpr struct {
	Pos   lexer.Position  `parser:""`
	Left  *CompareExpr    `@@`
	Right []*AndExprRight `( @@ )*`
}

type AndExprRight struct {
	Pos lexer.Position `parser:""`
	Op  string         `@( "and" )`
	Rhs *CompareExpr   `@@`
}

type CompareExpr struct {
	Pos   lexer.Position      `parser:""`
	Left  *AddExpr            `@@`
	Right []*CompareExprRight `( @@ )*`
}

type CompareExprRight struct {
	Pos lexer.Position `parser:""`
	Op  string         `@( "==" | "!=" | ">=" | "<=" | ">" | "<" )`
	Rhs *AddExpr       `@@`
}

type AddExpr struct {
	Pos   lexer.Position  `parser:""`
	Left  *MulExpr        `@@`
	Right []*AddExprRight `( @@ )*`
}

type AddExprRight struct {
	Pos lexer.Position `parser:""`
	Op  string         `@( "+" | "-" )`
	Rhs *MulExpr       `@@`
}

type MulExpr struct {
	Pos   lexer.Position  `parser:""`
	Left  *UnaryExpr      `@@`
	Right []*MulExprRight `( @@ )*`
}

type MulExprRight struct {
	Pos lexer.Position `parser:""`
	Op  string         `@( "*" | "/" )`
	Rhs *UnaryExpr     `@@`
}

type UnaryExpr struct {
	Pos  lexer.Position `parser:""`
	Neg  *string        `( @("-") )?`
	Prim *Primary       `  @@`
}

type Primary struct {
	Pos       lexer.Position `parser:""`
	BoolLit   *string        `  @( "true" | "false" )`
	Duration  *string        `| @Duration`
	StringLit *string        `| @String`
	Float     *string        `| @Float`
	Int       *string        `| @Int`
	Ident     *string        `| @Ident`
	SubExpr   *Expr          `| "(" @@ ")"`
}
