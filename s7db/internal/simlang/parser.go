package simlang

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var lex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `//[^\n]*`},
	{Name: "Whitespace", Pattern: `[ \t\r\n]+`},
	{Name: "Duration", Pattern: `(?:T#)?[0-9]+(?:ns|us|µs|ms|s|m|h)`},
	{Name: "Float", Pattern: `[0-9]+\.[0-9]+`},
	{Name: "Int", Pattern: `[0-9]+`},
	{Name: "Operator", Pattern: `:=|==|!=|>=|<=|[+\-*/><():]`},
	{Name: "Ident", Pattern: `[A-Za-z_][A-Za-z0-9_]*`},
})

var parser = participle.MustBuild[AST](
	participle.Lexer(lex),
	participle.Elide("Whitespace", "Comment"),
	participle.UseLookahead(2),
	participle.CaseInsensitive("true", "false", "if", "then", "else", "end", "on", "do", "var", "tick", "and", "or", "rising", "falling"),
)

func Parse(file, src string) (*AST, error) {
	ast, err := parser.ParseString(file, src)
	if err != nil {
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "unexpected token") {
			return nil, fmt.Errorf("parse error: %s", msg)
		}
		return nil, err
	}
	return ast, nil
}
