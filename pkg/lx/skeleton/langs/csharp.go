package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func CSharpEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	body := n.ChildByFieldName("body", lang)
	if body == nil {
		body = findChildByType(n, "declaration_list", lang)
	}
	if body == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if csIsPrivate(child, src, lang) {
			continue
		}
		switch child.Type(lang) {
		case "field_declaration", "property_declaration":
			out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "method_declaration", "constructor_declaration":
			if functions {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		case "class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "record_declaration":
			out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
			out = CSharpEmitClass(out, src, lines, child, lang, functions)
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}

func csIsPrivate(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type(lang) == "modifier" && strings.TrimSpace(child.Text(src)) == "private" {
			return true
		}
	}
	return false
}
