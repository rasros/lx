package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// RubyEmitClassOrModule emits a Ruby class or module. It is custom because
// the body has no delimiters and visibility is a bare `private` or
// `protected` call that hides every member after it.
func RubyEmitClassOrModule(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	body := n.ChildByFieldName("body", c.Lang)
	if body == nil {
		body = findChildByType(n, "body_statement", c.Lang)
	}
	if body == nil {
		if !c.Structs {
			return out
		}
		return c.KeepNode(out, n)
	}

	var inner []internal.LineRange
	isPrivate := false
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch c.Type(child) {
		case "identifier":
			text := strings.TrimSpace(c.Text(child))
			isPrivate = text == "private" || text == "protected"
		case "class", "module":
			if !isPrivate {
				if sub := RubyEmitClassOrModule(c, nil, child); len(sub) > 0 {
					inner = c.EmitLeadingDoc(inner, startRow(child))
					inner = append(inner, sub...)
				}
			}
		case "method", "singleton_method":
			if !isPrivate && c.Functions {
				inner = c.EmitLeadingDoc(inner, startRow(child))
				inner = EmitFuncSig(inner, c.Lines, child, c.Lang, true, "")
			}
		case "call", "assignment":
			if !isPrivate && c.Structs {
				inner = c.EmitLineWithDoc(inner, startRow(child))
			}
		}
	}
	if !c.Structs && len(inner) == 0 {
		return out
	}

	headerLine := startRow(n)
	out = internal.KeepLine(out, c.Lines, headerLine)
	out = append(out, inner...)
	if end := endRow(n); end > headerLine {
		out = internal.KeepLine(out, c.Lines, end)
	}
	return out
}
