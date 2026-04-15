package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func JavaEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	var body *gotreesitter.Node
	for _, bodyType := range []string{"class_body", "interface_body", "enum_body"} {
		if b := findChildByType(n, bodyType, lang); b != nil {
			body = b
			break
		}
	}
	if body == nil {
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)

	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if javaIsPrivate(child, src) {
			continue
		}
		switch child.Type(lang) {
		case "field_declaration":
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "method_declaration", "constructor_declaration":
			if functions {
				out = emitFuncSig(out, lines, child, lang, false)
			}
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
			out = JavaEmitClass(out, src, lines, child, lang, functions)
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func javaIsPrivate(n *gotreesitter.Node, src []byte) bool {
	return strings.HasPrefix(strings.TrimSpace(n.Text(src)), "private ")
}
