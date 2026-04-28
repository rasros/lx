package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func RustFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.ChildCount() > 0 && n.Child(0).Type(lang) == "visibility_modifier" {
		return strings.HasPrefix(n.Child(0).Text(src), "pub")
	}
	return false
}

func RustEmitItem(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	switch n.Type(lang) {
	case "struct_item":
		return rustEmitStruct(out, src, lines, n, lang)
	case "enum_item":
		return rustEmitEnum(out, src, lines, n, lang)
	case "trait_item":
		return rustEmitTrait(out, src, lines, n, lang, functions)
	case "impl_item":
		return rustEmitImpl(out, src, lines, n, lang, functions)
	}
	return out
}

func rustIsPublic(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.ChildCount() > 0 && n.Child(0).Type(lang) == "visibility_modifier" {
		return strings.HasPrefix(n.Child(0).Text(src), "pub")
	}
	return false
}

func rustEmitStruct(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	fieldList := findChildByType(n, "field_declaration_list", lang)
	if fieldList == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(fieldList.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(i)
		if child.Type(lang) == "field_declaration" && rustIsPublic(child, src, lang) {
			out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}
	if closingRow := int(fieldList.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}

func rustEmitEnum(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	variantList := findChildByType(n, "enum_variant_list", lang)
	if variantList == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(variantList.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < variantList.ChildCount(); i++ {
		child := variantList.Child(i)
		if child.Type(lang) == "enum_variant" {
			out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		}
	}
	if closingRow := int(variantList.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}

func rustEmitTrait(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	declList := findChildByType(n, "declaration_list", lang)
	if declList == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(declList.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)
	if functions {
		for i := 0; i < declList.ChildCount(); i++ {
			child := declList.Child(i)
			t := child.Type(lang)
			if t == "function_item" || t == "function_signature_item" {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		}
	}
	if closingRow := int(declList.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}

func rustEmitImpl(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	declList := findChildByType(n, "declaration_list", lang)
	if declList == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(declList.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)
	if functions {
		for i := 0; i < declList.ChildCount(); i++ {
			child := declList.Child(i)
			if child.Type(lang) == "function_item" && rustIsPublic(child, src, lang) {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		}
	}
	if closingRow := int(declList.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}
