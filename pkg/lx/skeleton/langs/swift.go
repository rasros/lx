package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func SwiftFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return !swiftIsPrivate(n, src, lang)
}

func SwiftEmitType(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	var body *gotreesitter.Node
	if n.Type(lang) == "protocol_declaration" {
		body = findChildByType(n, "protocol_body", lang)
	} else {
		body = findChildByType(n, "class_body", lang)
	}
	if body == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if swiftIsPrivate(child, src, lang) {
			continue
		}
		switch child.Type(lang) {
		case "property_declaration":
			out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "function_declaration", "init_declaration":
			if functions {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		case "class_declaration":
			out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
			out = SwiftEmitType(out, src, lines, child, lang, functions)
		case "protocol_function_declaration", "protocol_property_declaration":
			out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}

func swiftIsPrivate(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	mods := findChildByType(n, "modifiers", lang)
	if mods == nil {
		return false
	}
	for i := 0; i < mods.ChildCount(); i++ {
		child := mods.Child(i)
		if child.Type(lang) == "visibility_modifier" {
			text := strings.TrimSpace(child.Text(src))
			if text == "private" || text == "fileprivate" {
				return true
			}
		}
	}
	return false
}
