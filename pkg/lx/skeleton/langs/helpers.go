package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
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
		return internal.AppendLine(out, lines, startRow)
	}

	body := n.ChildByFieldName("body", lang)
	if body == nil && bodyChildType != "" {
		body = findChildByType(n, bodyChildType, lang)
	}
	if body == nil {
		return internal.AppendLines(out, lines, startRow, int(n.EndPoint().Row))
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
		endRow = sigEndRow(n, body, lang, startRow)
	}
	return internal.AppendLines(out, lines, startRow, endRow)
}

// sigEndRow returns the last row of n's signature, skipping any comment
// between the signature and the body.
func sigEndRow(n, body *gotreesitter.Node, lang *gotreesitter.Language, startRow int) int {
	end := startRow
	bodyStart := body.StartPoint()
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		cs := c.StartPoint()
		if cs.Row > bodyStart.Row || (cs.Row == bodyStart.Row && cs.Column >= bodyStart.Column) {
			continue
		}
		if strings.Contains(c.Type(lang), "comment") {
			continue
		}
		if r := int(c.EndPoint().Row); r > end {
			end = r
		}
	}
	return end
}

func emitFuncSig(out []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, indentBody bool, bodyChildType string) []byte {
	return EmitFuncSig(out, lines, n, lang, indentBody, bodyChildType, false)
}

// EmitDecorators appends the decorator/annotation lines in [fromRow, sigRow-1].
func EmitDecorators(out []byte, lines [][]byte, fromRow, sigRow int) []byte {
	if fromRow < sigRow {
		return internal.AppendLines(out, lines, fromRow, sigRow-1)
	}
	return out
}

// PrecedingAnnotationRow returns the start row of the contiguous decorator/
// annotation siblings before child (index i in body), or child's own start row.
func PrecedingAnnotationRow(body, child *gotreesitter.Node, i int, lang *gotreesitter.Language) int {
	row := int(child.StartPoint().Row)
	for j := i - 1; j >= 0; j-- {
		switch body.Child(j).Type(lang) {
		case "decorator", "annotation":
			row = int(body.Child(j).StartPoint().Row)
		default:
			return row
		}
	}
	return row
}

func emitAllLines(out []byte, lines [][]byte, n *gotreesitter.Node) []byte {
	return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
}
