package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func KtFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return !KotlinPrivate(&Ctx{Src: src, Lang: lang}, nil, n)
}

// KotlinPrivate reports a member with a private visibility modifier.
func KotlinPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	mods := findChildByType(n, "modifiers", c.Lang)
	if mods == nil {
		return false
	}
	for i := 0; i < mods.ChildCount(); i++ {
		child := mods.Child(i)
		if c.Type(child) == "visibility_modifier" && c.Text(child) == "private" {
			return true
		}
	}
	return false
}

// KotlinEmitter wraps the generic walker for Kotlin's one irregular shape:
// a top-level `object : Base { ... }` parses as an infix_expression whose
// members are the statements of a lambda_literal. Other infix expressions are
// not declarations and keep nothing.
func KotlinEmitter(m *Members) Emitter {
	return func(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
		if c.Type(n) != "infix_expression" {
			return m.Emit(c, out, n)
		}
		if first := n.Child(0); first == nil || c.Type(first) != "object_literal" {
			return out
		}
		body := findChildByType(n, "lambda_literal", c.Lang)
		if body == nil {
			if !c.Structs {
				return out
			}
			return c.KeepNode(out, n)
		}
		items := findChildByType(body, "statements", c.Lang)
		if items == nil {
			items = body // nothing inside: header and closing row only
		}
		return m.Walk(c, out, n, body, items)
	}
}
