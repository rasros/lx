package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func PyFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	name := n.ChildByFieldName("name", lang)
	return name != nil && !isPrivateName(name.Text(src))
}

func PyEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	nameNode := n.ChildByFieldName("name", lang)
	if nameNode == nil || isPrivateName(nameNode.Text(src)) {
		return out
	}
	bodyNode := n.ChildByFieldName("body", lang)
	if bodyNode == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	out = internal.AppendLines(out, lines, int(n.StartPoint().Row), int(bodyNode.StartPoint().Row)-1)
	out = AppendPyDocstring(out, lines, n, lang)
	for i := 0; i < bodyNode.ChildCount(); i++ {
		out = pyProcessMember(out, src, lines, bodyNode.Child(i), lang, functions)
	}
	return out
}

func pyProcessMember(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	actual := n
	if n.Type(lang) == "decorated_definition" {
		if defNode := n.ChildByFieldName("definition", lang); defNode != nil {
			actual = defNode
		}
	}
	outerStart := int(n.StartPoint().Row)
	switch actual.Type(lang) {
	case "function_definition":
		if !functions {
			return out
		}
		nameNode := actual.ChildByFieldName("name", lang)
		if nameNode == nil || isPrivateName(nameNode.Text(src)) {
			return out
		}
		out = EmitLeadingDoc(out, lines, outerStart)
		out = EmitDecorators(out, lines, outerStart, int(actual.StartPoint().Row))
		out = emitFuncSig(out, lines, actual, lang, true, "")
		out = AppendPyDocstring(out, lines, actual, lang)
		return out
	case "class_definition":
		out = EmitLeadingDoc(out, lines, outerStart)
		out = EmitDecorators(out, lines, outerStart, int(actual.StartPoint().Row))
		return PyEmitClass(out, src, lines, actual, lang, functions)
	default:
		if pyIsClassVar(n, lang) {
			name := pyVarName(n, lines)
			if name != "" && !isPrivateName(name) {
				return EmitLineWithDoc(out, lines, int(n.StartPoint().Row))
			}
		}
	}
	return out
}

func pyIsClassVar(n *gotreesitter.Node, lang *gotreesitter.Language) bool {
	t := n.Type(lang)
	if t == "assignment" || t == "annotated_assignment" || t == "augmented_assignment" {
		return true
	}
	if t == "expression_statement" && n.ChildCount() > 0 {
		ct := n.Child(0).Type(lang)
		return ct == "assignment" || ct == "annotated_assignment" || ct == "augmented_assignment"
	}
	return false
}

func pyVarName(n *gotreesitter.Node, lines [][]byte) string {
	row := int(n.StartPoint().Row)
	if row >= len(lines) {
		return ""
	}
	line := lines[row]
	start := 0
	for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	stripped := line[start:]
	for i, ch := range string(stripped) {
		if ch == '=' || ch == ':' {
			return strings.TrimSpace(string(stripped[:i]))
		}
	}
	return ""
}
