package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func isUppercase(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

func isPrivateName(name string) bool {
	return len(name) > 0 && name[0] == '_' && (len(name) < 2 || name[1] != '_')
}

func findChildByType(n *gotreesitter.Node, typ string, lang *gotreesitter.Language) *gotreesitter.Node {
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type(lang) == typ {
			return child
		}
	}
	return nil
}

func hasDescendantOfType(n *gotreesitter.Node, typ string, lang *gotreesitter.Language) bool {
	if n.Type(lang) == typ {
		return true
	}
	for i := 0; i < n.ChildCount(); i++ {
		if hasDescendantOfType(n.Child(i), typ, lang) {
			return true
		}
	}
	return false
}

// EmitFuncSig appends a function signature (without body) to out.
func EmitFuncSig(out []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, indentBody bool, bodyChildType string, singleLineSig bool) []byte {
	startRow := int(n.StartPoint().Row)
	if singleLineSig {
		return content.AppendLine(out, lines, startRow)
	}

	body := n.ChildByFieldName("body", lang)
	if body == nil && bodyChildType != "" {
		body = findChildByType(n, bodyChildType, lang)
	}
	if body == nil {
		return content.AppendLines(out, lines, startRow, int(n.EndPoint().Row))
	}

	bodyStartRow := int(body.StartPoint().Row)
	bodyEndRow := int(body.EndPoint().Row)
	if bodyStartRow == startRow && bodyEndRow == bodyStartRow && startRow >= 0 && startRow < len(lines) {
		line := lines[startRow]
		col := int(body.StartPoint().Column)
		if col < 0 {
			col = 0
		}
		if col > len(line) {
			col = len(line)
		}
		endCol := col
		if col < len(line) && line[col] == '{' {
			endCol = col + 1
		}
		out = append(out, line[:endCol]...)
		out = append(out, '\n')
		return out
	}

	endRow := bodyStartRow
	if indentBody {
		endRow--
	}
	return content.AppendLines(out, lines, startRow, endRow)
}

func emitFuncSig(out []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, indentBody bool, bodyChildType string) []byte {
	return EmitFuncSig(out, lines, n, lang, indentBody, bodyChildType, false)
}

func emitAllLines(out []byte, lines [][]byte, n *gotreesitter.Node) []byte {
	return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
}
