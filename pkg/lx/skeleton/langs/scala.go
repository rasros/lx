package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func ScalaFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return !scalaIsPrivate(n, src, lang)
}

func ScalaEmitType(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	body := findChildByType(n, "template_body", lang)
	if body == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerStart := int(n.StartPoint().Row)
	headerEnd := int(body.StartPoint().Row)
	out = internal.AppendLines(out, lines, headerStart, headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if scalaIsPrivate(child, src, lang) {
			continue
		}
		switch child.Type(lang) {
		case "val_definition", "var_definition":
			out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "function_definition", "function_declaration":
			if functions {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, false, "")
			}
		case "class_definition", "object_definition", "trait_definition":
			out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
			out = ScalaEmitType(out, src, lines, child, lang, functions)
		}
	}
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}

func scalaIsPrivate(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	mods := findChildByType(n, "modifiers", lang)
	if mods == nil {
		return false
	}
	for i := 0; i < mods.ChildCount(); i++ {
		child := mods.Child(i)
		if child.Type(lang) == "access_modifier" {
			return strings.HasPrefix(strings.TrimSpace(child.Text(src)), "private")
		}
	}
	return false
}
