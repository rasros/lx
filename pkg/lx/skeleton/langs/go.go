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
	grouped := goIsGroupedTypeDecl(n, lang)
	groupMembers := make([]byte, 0)

	for i := 0; i < n.ChildCount(); i++ {
		spec := n.Child(i)
		if spec.Type(lang) != "type_spec" {
			continue
		}

		if grouped {
			groupMembers = goEmitTypeSpec(groupMembers, src, lines, spec, lang, functions)
		} else {
			out = goEmitTypeSpec(out, src, lines, spec, lang, functions)
		}
	}

	if grouped && len(groupMembers) > 0 {
		out = content.AppendLine(out, lines, int(n.StartPoint().Row))
		out = append(out, groupMembers...)
		out = content.AppendLine(out, lines, int(n.EndPoint().Row))
	}
	return out
}

func goEmitTypeSpec(out, src []byte, lines [][]byte, spec *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	nameNode := spec.ChildByFieldName("name", lang)
	if nameNode == nil || !isUppercase(nameNode.Text(src)) {
		return out
	}

	typeNode := spec.ChildByFieldName("type", lang)
	if typeNode == nil {
		return out
	}

	switch typeNode.Type(lang) {
	case "struct_type":
		return goEmitStruct(out, src, lines, spec, typeNode, lang)
	case "interface_type":
		return goEmitInterface(out, src, lines, spec, typeNode, lang, functions)
	default:
		return content.AppendLines(out, lines, int(spec.StartPoint().Row), int(spec.EndPoint().Row))
	}
}

func goEmitStruct(out, src []byte, lines [][]byte, specNode, structNode *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	fieldList := findChildByType(structNode, "field_declaration_list", lang)
	if fieldList == nil {
		return content.AppendLines(out, lines, int(specNode.StartPoint().Row), int(specNode.EndPoint().Row))
	}
	headerStart := int(specNode.StartPoint().Row)
	out = content.AppendLine(out, lines, headerStart)

	for i := 0; i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(i)
		if child.Type(lang) == "field_declaration" && goFieldIsExported(child, src, lang) && int(child.StartPoint().Row) != headerStart {
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}

	if closingRow := int(fieldList.EndPoint().Row); closingRow > headerStart {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func goEmitInterface(out, src []byte, lines [][]byte, specNode, ifaceNode *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	headerStart := int(specNode.StartPoint().Row)
	out = content.AppendLine(out, lines, headerStart)

	if functions {
		for i := 0; i < ifaceNode.ChildCount(); i++ {
			child := ifaceNode.Child(i)
			t := child.Type(lang)
			if t != "method_elem" && t != "method_spec" {
				continue
			}
			nameNode := findChildByType(child, "field_identifier", lang)
			if nameNode != nil && isUppercase(nameNode.Text(src)) && int(child.StartPoint().Row) != headerStart {
				out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
			}
		}
	}

	if closingRow := int(ifaceNode.EndPoint().Row); closingRow > headerStart {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func goIsGroupedTypeDecl(n *gotreesitter.Node, lang *gotreesitter.Language) bool {
	for i := 0; i < n.ChildCount(); i++ {
		if n.Child(i).Type(lang) == "(" {
			return true
		}
	}
	return false
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
