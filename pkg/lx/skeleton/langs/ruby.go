package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func RubyEmitClassOrModule(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	body := n.ChildByFieldName("body", lang)
	if body == nil {
		body = findChildByType(n, "body_statement", lang)
	}
	if body == nil {
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	// Emit just the header line (class/module declaration)
	headerLine := int(n.StartPoint().Row)
	out = content.AppendLine(out, lines, headerLine)

	isPrivate := false
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Type(lang) {
		case "identifier":
			text := strings.TrimSpace(child.Text(src))
			isPrivate = text == "private" || text == "protected"
		case "class", "module":
			if !isPrivate {
				out = RubyEmitClassOrModule(out, src, lines, child, lang, functions)
			}
		case "method", "singleton_method":
			if !isPrivate && functions {
				out = emitFuncSig(out, lines, child, lang, true, "")
			}
		case "call":
			// attr_reader, attr_writer, attr_accessor, etc.
			if !isPrivate {
				out = content.AppendLine(out, lines, int(child.StartPoint().Row))
			}
		case "assignment":
			// Class constants (e.g., MAX = 100)
			if !isPrivate {
				out = content.AppendLine(out, lines, int(child.StartPoint().Row))
			}
		}
	}
	// Emit closing "end"
	endRow := int(n.EndPoint().Row)
	if endRow > headerLine {
		out = content.AppendLine(out, lines, endRow)
	}
	return out
}
