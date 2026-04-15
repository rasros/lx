package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func PascalEmitDecl(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	switch n.Type(lang) {
	case "declTypes":
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	case "defProc":
		if !functions {
			return out
		}
		body := n.ChildByFieldName("body", lang)
		startRow := int(n.StartPoint().Row)
		if body == nil {
			return content.AppendLine(out, lines, startRow)
		}
		endRow := int(body.StartPoint().Row) - 1
		if endRow < startRow {
			endRow = startRow
		}
		return content.AppendLines(out, lines, startRow, endRow)
	}
	return out
}
