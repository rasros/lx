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

// TSEmitFunc handles top-level function_declaration and lexical_declaration (arrow/const functions).
func TSEmitFunc(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	if n.Type(lang) == "function_declaration" {
		return emitFuncSig(out, lines, n, lang, false, "")
	}
	// lexical_declaration: emit only if it wraps an arrow_function or function_expression.
	decl := findChildByType(n, "variable_declarator", lang)
	if decl == nil {
		return out
	}
	val := decl.ChildByFieldName("value", lang)
	if val == nil {
		return out
	}
	valType := val.Type(lang)
	if valType != "arrow_function" && valType != "function_expression" {
		return out
	}
	body := val.ChildByFieldName("body", lang)
	startRow := int(n.StartPoint().Row)
	if body != nil && body.Type(lang) == "statement_block" {
		return content.AppendLines(out, lines, startRow, int(body.StartPoint().Row))
	}
	return content.AppendLines(out, lines, startRow, int(n.EndPoint().Row))
}
