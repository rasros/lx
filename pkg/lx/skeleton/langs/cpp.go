package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func CppFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.Type(lang) == "declaration" {
		return hasDescendantOfType(n, "function_declarator", lang)
	}
	return true
}

func CppEmitClassOrStruct(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	if n.Type(lang) == "type_definition" {
		return CEmitTypedef(out, src, lines, n, lang, functions)
	}

	fieldList := findChildByType(n, "field_declaration_list", lang)
	if fieldList == nil {
		return out
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(fieldList.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)

	isPrivateSection := n.Type(lang) == "class_specifier"

	for i := 0; i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(i)
		if child.Type(lang) == "access_specifier" {
			isPrivateSection = strings.TrimSpace(child.Text(src)) == "private:"
			continue
		}
		if isPrivateSection {
			continue
		}
		switch child.Type(lang) {
		case "field_declaration":
			if !hasDescendantOfType(child, "function_declarator", lang) {
				out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
			}
		case "function_definition":
			if functions {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		case "declaration":
			if hasDescendantOfType(child, "function_declarator", lang) {
				if functions {
					out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
				}
			} else {
				out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
			}
		}
	}
	if closingRow := int(fieldList.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}
