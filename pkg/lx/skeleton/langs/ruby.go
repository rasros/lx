package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func RubyEmitClassOrModule(out []internal.LineRange, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []internal.LineRange {
	body := n.ChildByFieldName("body", lang)
	if body == nil {
		body = findChildByType(n, "body_statement", lang)
	}
	if body == nil {
		return internal.KeepLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerLine := int(n.StartPoint().Row)
	out = internal.KeepLine(out, lines, headerLine)

	isPrivate := false
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Type(lang) {
		case "identifier":
			text := strings.TrimSpace(child.Text(src))
			isPrivate = text == "private" || text == "protected"
		case "class", "module":
			if !isPrivate {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = RubyEmitClassOrModule(out, src, lines, child, lang, functions)
			}
		case "method", "singleton_method":
			if !isPrivate && functions {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, true, "")
			}
		case "call", "assignment":
			if !isPrivate {
				out = EmitLineWithDoc(out, lines, int(child.StartPoint().Row))
			}
		}
	}
	endRow := int(n.EndPoint().Row)
	if endRow > headerLine {
		out = internal.KeepLine(out, lines, endRow)
	}
	return out
}
