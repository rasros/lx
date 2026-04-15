package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func CFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.Type(lang) == "declaration" {
		return hasDescendantOfType(n, "function_declarator", lang)
	}
	return true
}

func CEmitTypedef(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, _ bool) []byte {
	var specifier *gotreesitter.Node
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		t := child.Type(lang)
		if t == "struct_specifier" || t == "union_specifier" {
			specifier = child
			break
		}
	}
	if specifier == nil {
		return out
	}
	fieldList := specifier.ChildByFieldName("body", lang)
	if fieldList == nil {
		return out
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(fieldList.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(i)
		if child.Type(lang) == "field_declaration" {
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}
	if closingRow := int(n.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}
