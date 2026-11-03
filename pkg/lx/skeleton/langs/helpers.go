// Package langs holds the per-language skeleton emitters. An emitter
// appends the source rows it keeps as internal.LineRanges, in any order,
// and the skeleton package merges and renders them.
package langs

import (
	"slices"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// Ctx is the file being reduced to a skeleton and what to keep of it.
type Ctx struct {
	Src      []byte
	Lines    [][]byte
	Lang     *gotreesitter.Language
	Comments *CommentSyntax

	Functions bool
	Structs   bool
}

// Emitter appends the rows kept of node n to out.
type Emitter func(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange

// Type returns the node type of n.
func (c *Ctx) Type(n *gotreesitter.Node) string { return n.Type(c.Lang) }

// Text returns the source text of n.
func (c *Ctx) Text(n *gotreesitter.Node) string { return n.Text(c.Src) }

// KeepNode appends all rows of n.
func (c *Ctx) KeepNode(out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	return internal.KeepLines(out, c.Lines, startRow(n), endRow(n))
}

func startRow(n *gotreesitter.Node) int { return int(n.StartPoint().Row) }
func endRow(n *gotreesitter.Node) int   { return int(n.EndPoint().Row) }

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

// EmitFuncSig appends the rows of a function signature (without body) to out.
// A body that starts and ends on the signature's own line is cut from it,
// keeping an opening brace.
func EmitFuncSig(out []internal.LineRange, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, indentBody bool, bodyChildType string) []internal.LineRange {
	startRow := int(n.StartPoint().Row)

	body := n.ChildByFieldName("body", lang)
	if body == nil && bodyChildType != "" {
		body = findChildByType(n, bodyChildType, lang)
	}
	if body == nil {
		return internal.KeepLines(out, lines, startRow, int(n.EndPoint().Row))
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
		return internal.KeepPrefix(out, lines, startRow, endCol)
	}

	endRow := bodyStartRow
	if indentBody {
		endRow = sigEndRow(n, body, lang, startRow)
	}
	return internal.KeepLines(out, lines, startRow, endRow)
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

// EmitDecorators appends the decorator/annotation lines in [fromRow, sigRow-1].
func EmitDecorators(out []internal.LineRange, lines [][]byte, fromRow, sigRow int) []internal.LineRange {
	if fromRow < sigRow {
		return internal.KeepLines(out, lines, fromRow, sigRow-1)
	}
	return out
}

// precedingSiblingsRow returns the start row of the contiguous siblings of one
// of types right before child (index i in parent), or child's own start row.
func precedingSiblingsRow(parent, child *gotreesitter.Node, i int, types []string, lang *gotreesitter.Language) int {
	row := startRow(child)
	for j := i - 1; j >= 0; j-- {
		if !slices.Contains(types, parent.Child(j).Type(lang)) {
			break
		}
		row = startRow(parent.Child(j))
	}
	return row
}
