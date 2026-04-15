package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
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
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	out = content.AppendLines(out, lines, int(n.StartPoint().Row), int(bodyNode.StartPoint().Row)-1)
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
	switch actual.Type(lang) {
	case "function_definition":
		if !functions {
			return out
		}
		nameNode := actual.ChildByFieldName("name", lang)
		if nameNode == nil || isPrivateName(nameNode.Text(src)) {
			return out
		}
		return emitFuncSig(out, lines, actual, lang, true, "")
	case "class_definition":
		return PyEmitClass(out, src, lines, actual, lang, functions)
	default:
		if pyIsClassVar(n, lang) {
			name := pyVarName(n, lines)
			if name != "" && !isPrivateName(name) {
				return content.AppendLine(out, lines, int(n.StartPoint().Row))
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
