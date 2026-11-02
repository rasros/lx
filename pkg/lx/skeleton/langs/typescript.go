package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// TSPrivate reports a class member with the private accessibility modifier
// or an ECMAScript private name (#field, #method()).
func TSPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	if name := n.ChildByFieldName("name", c.Lang); name != nil && strings.HasPrefix(c.Text(name), "#") {
		return true
	}
	acc := findChildByType(n, "accessibility_modifier", c.Lang)
	return acc != nil && strings.TrimSpace(c.Text(acc)) == "private"
}

// TSEmitFunc handles top-level function_declaration and lexical_declaration (arrow/const functions).
func TSEmitFunc(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	if c.Type(n) == "function_declaration" {
		return EmitFuncSig(out, c.Lines, n, c.Lang, false, "")
	}
	decl := findChildByType(n, "variable_declarator", c.Lang)
	if decl == nil {
		return out
	}
	val := decl.ChildByFieldName("value", c.Lang)
	if val == nil {
		return out
	}
	valType := c.Type(val)
	if valType != "arrow_function" && valType != "function_expression" {
		return out
	}
	body := val.ChildByFieldName("body", c.Lang)
	if body != nil && c.Type(body) == "statement_block" {
		return internal.KeepLines(out, c.Lines, startRow(n), startRow(body))
	}
	return c.KeepNode(out, n)
}
