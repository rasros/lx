package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func GoFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	name := n.ChildByFieldName("name", lang)
	return name != nil && isUppercase(name.Text(src))
}

func GoEmitTypeDecl(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	for i := 0; i < n.ChildCount(); i++ {
		spec := n.Child(i)
		if spec.Type(lang) != "type_spec" {
			continue
		}
		nameNode := spec.ChildByFieldName("name", lang)
		if nameNode == nil || !isUppercase(nameNode.Text(src)) {
			continue
		}
		typeNode := spec.ChildByFieldName("type", lang)
		if typeNode == nil {
			continue
		}
		switch typeNode.Type(lang) {
		case "struct_type":
			out = goEmitStruct(out, src, lines, n, typeNode, lang)
		case "interface_type":
			out = goEmitInterface(out, src, lines, n, typeNode, lang, functions)
		default:
			out = content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
		}
	}
	return out
}

func goEmitStruct(out, src []byte, lines [][]byte, typeDeclNode, structNode *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	fieldList := findChildByType(structNode, "field_declaration_list", lang)
	if fieldList == nil {
		return content.AppendLines(out, lines, int(typeDeclNode.StartPoint().Row), int(typeDeclNode.EndPoint().Row))
	}
	headerStart := int(typeDeclNode.StartPoint().Row)
	headerEnd := int(fieldList.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(i)
		if child.Type(lang) == "field_declaration" && goFieldIsExported(child, src, lang) {
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}
	if closingRow := int(fieldList.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func goEmitInterface(out, src []byte, lines [][]byte, typeDeclNode, ifaceNode *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	headerStart := int(typeDeclNode.StartPoint().Row)
	headerEnd := int(ifaceNode.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)
	if functions {
		for i := 0; i < ifaceNode.ChildCount(); i++ {
			child := ifaceNode.Child(i)
			t := child.Type(lang)
			if t != "method_elem" && t != "method_spec" {
				continue
			}
			nameNode := findChildByType(child, "field_identifier", lang)
			if nameNode != nil && isUppercase(nameNode.Text(src)) {
				out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
			}
		}
	}
	if closingRow := int(ifaceNode.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func goFieldIsExported(field *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if name := field.ChildByFieldName("name", lang); name != nil {
		return isUppercase(name.Text(src))
	}
	for i := 0; i < field.ChildCount(); i++ {
		child := field.Child(i)
		t := child.Type(lang)
		if t == "field_identifier" || t == "type_identifier" {
			text := strings.TrimLeft(child.Text(src), "*")
			if dot := strings.LastIndex(text, "."); dot >= 0 {
				text = text[dot+1:]
			}
			return isUppercase(text)
		}
	}
	return false
}
