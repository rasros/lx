package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

// TSEmitClassOrInterface handles TypeScript/JavaScript class and interface declarations.
func TSEmitClassOrInterface(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	if n.Type(lang) == "interface_declaration" {
		return tsEmitInterface(out, src, lines, n, lang)
	}
	return tsEmitClass(out, src, lines, n, lang, functions)
}

func tsEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	body := findChildByType(n, "class_body", lang)
	if body == nil {
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if tsIsPrivate(child, src, lang) {
			continue
		}
		switch child.Type(lang) {
		case "public_field_definition", "field_definition":
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "method_definition":
			if functions {
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func tsEmitInterface(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	body := n.ChildByFieldName("body", lang)
	if body == nil {
		body = findChildByType(n, "interface_body", lang)
	}
	if body == nil {
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Type(lang) {
		case "method_signature", "property_signature", "call_signature", "construct_signature", "index_signature":
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func tsIsPrivate(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	acc := findChildByType(n, "accessibility_modifier", lang)
	if acc != nil {
		return strings.TrimSpace(acc.Text(src)) == "private"
	}
	return false
}
