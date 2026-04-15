package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func KtFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return !ktIsPrivate(n, src, lang)
}

func KotlinEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	// Kotlin object declarations parse as infix_expression with object_literal first child.
	if n.Type(lang) == "infix_expression" {
		first := n.Child(0)
		if first == nil || first.Type(lang) != "object_literal" {
			return out
		}
		return kotlinEmitObject(out, src, lines, n, lang, functions)
	}

	body := findChildByType(n, "class_body", lang)
	if body == nil {
		// Compact declaration (e.g., data class with only primary constructor)
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = content.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if ktIsPrivate(child, src, lang) {
			continue
		}
		switch child.Type(lang) {
		case "property_declaration":
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "function_declaration":
			if functions {
				out = emitFuncSig(out, lines, child, lang, false, "function_body")
			}
		case "class_declaration":
			out = KotlinEmitClass(out, src, lines, child, lang, functions)
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func kotlinEmitObject(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	body := findChildByType(n, "lambda_literal", lang)
	if body == nil {
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerEnd := int(body.StartPoint().Row)
	out = content.AppendLines(out, lines, int(n.StartPoint().Row), headerEnd)

	stmts := findChildByType(body, "statements", lang)
	if stmts != nil {
		for i := 0; i < stmts.ChildCount(); i++ {
			child := stmts.Child(i)
			if ktIsPrivate(child, src, lang) {
				continue
			}
			switch child.Type(lang) {
			case "property_declaration":
				out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
			case "function_declaration":
				if functions {
					out = emitFuncSig(out, lines, child, lang, false, "function_body")
				}
			}
		}
	}

	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

func ktIsPrivate(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	mods := findChildByType(n, "modifiers", lang)
	if mods == nil {
		return false
	}
	for i := 0; i < mods.ChildCount(); i++ {
		child := mods.Child(i)
		if child.Type(lang) == "visibility_modifier" {
			if child.Text(src) == "private" {
				return true
			}
		}
	}
	return false
}
