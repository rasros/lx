package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func PyFuncVisible(c *Ctx, n *gotreesitter.Node) bool {
	name := n.ChildByFieldName("name", c.Lang)
	return name != nil && !isPrivateName(c.Text(name))
}

// PyEmitClass emits a Python class. It is custom because the body is
// indentation, with no closing row, and visibility is a "_" name prefix.
func PyEmitClass(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	nameNode := n.ChildByFieldName("name", c.Lang)
	if nameNode == nil || isPrivateName(c.Text(nameNode)) {
		return out
	}
	bodyNode := n.ChildByFieldName("body", c.Lang)
	if bodyNode == nil {
		if !c.Structs {
			return out
		}
		return c.KeepNode(out, n)
	}
	var inner []internal.LineRange
	for i := 0; i < bodyNode.ChildCount(); i++ {
		inner = pyProcessMember(c, inner, bodyNode.Child(i))
	}
	if !c.Structs && len(inner) == 0 {
		return out
	}
	out = internal.KeepLines(out, c.Lines, startRow(n), startRow(bodyNode)-1)
	out = AppendPyDocstring(out, c.Lines, n, c.Lang)
	return append(out, inner...)
}

func pyProcessMember(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	actual := n
	if c.Type(n) == "decorated_definition" {
		if defNode := n.ChildByFieldName("definition", c.Lang); defNode != nil {
			actual = defNode
		}
	}
	outerStart := startRow(n)
	switch c.Type(actual) {
	case "function_definition":
		if !c.Functions {
			return out
		}
		nameNode := actual.ChildByFieldName("name", c.Lang)
		if nameNode == nil || isPrivateName(c.Text(nameNode)) {
			return out
		}
		out = c.EmitLeadingDoc(out, outerStart)
		out = EmitDecorators(out, c.Lines, outerStart, startRow(actual))
		out = EmitFuncSig(out, c.Lines, actual, c.Lang, true, "")
		return AppendPyDocstring(out, c.Lines, actual, c.Lang)
	case "class_definition":
		sub := PyEmitClass(c, nil, actual)
		if len(sub) == 0 {
			return out
		}
		out = c.EmitLeadingDoc(out, outerStart)
		out = EmitDecorators(out, c.Lines, outerStart, startRow(actual))
		return append(out, sub...)
	default:
		if c.Structs && pyIsClassVar(c, n) {
			name := pyVarName(n, c.Lines)
			if name != "" && !isPrivateName(name) {
				return c.EmitLineWithDoc(out, startRow(n))
			}
		}
	}
	return out
}

func pyIsClassVar(c *Ctx, n *gotreesitter.Node) bool {
	t := c.Type(n)
	if t == "assignment" || t == "annotated_assignment" || t == "augmented_assignment" {
		return true
	}
	if t == "expression_statement" && n.ChildCount() > 0 {
		ct := c.Type(n.Child(0))
		return ct == "assignment" || ct == "annotated_assignment" || ct == "augmented_assignment"
	}
	return false
}

func pyVarName(n *gotreesitter.Node, lines [][]byte) string {
	row := startRow(n)
	if row >= len(lines) {
		return ""
	}
	stripped := trimLeadingSpace(lines[row])
	for i, ch := range string(stripped) {
		if ch == '=' || ch == ':' {
			return strings.TrimSpace(string(stripped[:i]))
		}
	}
	return ""
}
