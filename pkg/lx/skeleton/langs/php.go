package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func PHPEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	isInterface := n.Type(lang) == "interface_declaration"
	body := findChildByType(n, "declaration_list", lang)
	if body == nil {
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if !isInterface && phpIsPrivate(child, src, lang) {
			continue
		}
		switch child.Type(lang) {
		case "property_declaration":
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "method_declaration":
			if functions {
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		case "const_declaration":
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func phpIsPrivate(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	vis := findChildByType(n, "visibility_modifier", lang)
	if vis == nil {
		return false
	}
	return strings.TrimSpace(vis.Text(src)) == "private"
}
