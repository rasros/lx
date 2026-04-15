package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func OCamlEmitDefinition(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	switch n.Type(lang) {
	case "type_definition":
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	case "value_definition":
		if !functions {
			return out
		}
		binding := findChildByType(n, "let_binding", lang)
		if binding == nil {
			return content.AppendLine(out, lines, int(n.StartPoint().Row))
		}
		body := binding.ChildByFieldName("body", lang)
		if body == nil {
			return content.AppendLine(out, lines, int(n.StartPoint().Row))
		}
		sigRow := int(n.StartPoint().Row)
		bodyRow := int(body.StartPoint().Row)
		if bodyRow > sigRow {
			return content.AppendLines(out, lines, sigRow, bodyRow-1)
		}
		return content.AppendLine(out, lines, sigRow)
	}
	return out
}
