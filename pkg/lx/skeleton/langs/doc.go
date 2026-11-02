package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// CommentSyntax says which lines above a declaration make up its leading
// doc block in one language: its comments, and attributes written on lines
// of their own outside the declaration's syntax node.
type CommentSyntax struct {
	// Prefixes are line starts (after indentation) that continue a doc
	// block: line comments, block comment openers and "*" continuation or
	// closing lines, and attribute markers such as Rust's "#".
	Prefixes []string
	// Blocks are multi-line comments: a line that ends one is followed up
	// to the line that opens it.
	Blocks []BlockComment
}

// BlockComment is a multi-line comment's delimiters.
type BlockComment struct {
	Open, Close string
	// WholeLine delimiters stand alone on their line, as Ruby's =begin and
	// =end do.
	WholeLine bool
}

var (
	// CComments is // and /* */ with "*" continuation lines, the comment
	// syntax of the brace languages. Preprocessor lines (#include, #define,
	// #region) are not comments.
	CComments = CommentSyntax{
		Prefixes: []string{"//", "/*", "*"},
		Blocks:   []BlockComment{{Open: "/*", Close: "*/"}},
	}
	// RustComments is CComments plus "#" attribute lines such as
	// #[derive(Debug)], which are sibling nodes of the item they annotate.
	RustComments = CommentSyntax{
		Prefixes: []string{"//", "/*", "*", "#"},
		Blocks:   CComments.Blocks,
	}
	// PHPComments is CComments plus # comments and #[Attribute] lines.
	PHPComments = RustComments
	// ZigComments is // only: Zig has no block comments.
	ZigComments = CommentSyntax{Prefixes: []string{"//"}}
	// HashComments is # line comments (Python).
	HashComments = CommentSyntax{Prefixes: []string{"#"}}
	// RubyComments is # and =begin/=end.
	RubyComments = CommentSyntax{
		Prefixes: []string{"#", "=begin"},
		Blocks:   []BlockComment{{Open: "=begin", Close: "=end", WholeLine: true}},
	}
	// HaskellComments is -- and {- -}.
	HaskellComments = CommentSyntax{
		Prefixes: []string{"--", "{-", "-}"},
		Blocks:   []BlockComment{{Open: "{-", Close: "-}"}},
	}
	// OCamlComments is (* *) with "*" continuation lines.
	OCamlComments = CommentSyntax{
		Prefixes: []string{"(*", "*"},
		Blocks:   []BlockComment{{Open: "(*", Close: "*)"}},
	}
)

// LeadingDocStart returns the first row of a contiguous comment block
// immediately preceding row, or row itself if there is none. A blank line
// terminates the block.
func (cs *CommentSyntax) LeadingDocStart(lines [][]byte, row int) int {
	r := row - 1
	for r >= 0 {
		line := lines[r]
		if isBlankLine(line) {
			break
		}
		if cs.isDocLine(line) {
			r--
			continue
		}
		block, ok := cs.closedBlock(line)
		if !ok {
			break
		}
		found := false
		for r >= 0 {
			if block.opens(lines[r]) {
				r--
				found = true
				break
			}
			r--
		}
		if !found {
			break
		}
	}
	if r+1 < row {
		return r + 1
	}
	return row
}

// EmitLeadingDoc appends only the leading doc block above row, if any.
func (c *Ctx) EmitLeadingDoc(out []internal.LineRange, row int) []internal.LineRange {
	docStart := c.Comments.LeadingDocStart(c.Lines, row)
	if docStart < row {
		return internal.KeepLines(out, c.Lines, docStart, row-1)
	}
	return out
}

// EmitWithDoc appends the leading doc block above startRow and the rows
// startRow..endRow.
func (c *Ctx) EmitWithDoc(out []internal.LineRange, startRow, endRow int) []internal.LineRange {
	return internal.KeepLines(out, c.Lines, c.Comments.LeadingDocStart(c.Lines, startRow), endRow)
}

// EmitLineWithDoc appends the leading doc block plus row itself.
func (c *Ctx) EmitLineWithDoc(out []internal.LineRange, row int) []internal.LineRange {
	return c.EmitWithDoc(out, row, row)
}

// AppendPyDocstring appends the Python docstring (if any) of n's body to out.
func AppendPyDocstring(out []internal.LineRange, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language) []internal.LineRange {
	start, end := PyDocstring(n, lang)
	if start < 0 {
		return out
	}
	return internal.KeepLines(out, lines, start, end)
}

// PyDocstring returns the row range of a Python docstring (the first string
// statement in the node's body), or (-1, -1) if there is none.
func PyDocstring(n *gotreesitter.Node, lang *gotreesitter.Language) (int, int) {
	body := n.ChildByFieldName("body", lang)
	if body == nil {
		return -1, -1
	}
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if !child.IsNamed() {
			continue
		}
		switch child.Type(lang) {
		case "string":
			return int(child.StartPoint().Row), int(child.EndPoint().Row)
		case "expression_statement":
			if child.ChildCount() > 0 && child.Child(0).Type(lang) == "string" {
				return int(child.StartPoint().Row), int(child.EndPoint().Row)
			}
		}
		return -1, -1
	}
	return -1, -1
}

func (cs *CommentSyntax) isDocLine(line []byte) bool {
	s := trimLeadingSpace(line)
	if len(s) == 0 {
		return false
	}
	for _, p := range cs.Prefixes {
		if hasPrefix(s, p) {
			return true
		}
	}
	return false
}

// closedBlock returns the block comment that line ends, if any.
func (cs *CommentSyntax) closedBlock(line []byte) (BlockComment, bool) {
	end := trimTrailingSpace(line)
	for _, b := range cs.Blocks {
		if b.WholeLine {
			if equalTrimmed(line, b.Close) {
				return b, true
			}
			continue
		}
		if !hasSuffix(end, b.Close) {
			continue
		}
		// Self-contained inline (e.g. `int x; /* note */`) is not a multi-line closer.
		if bytesContains(end[:len(end)-len(b.Close)], b.Open) {
			return BlockComment{}, false
		}
		return b, true
	}
	return BlockComment{}, false
}

func (b BlockComment) opens(line []byte) bool {
	if b.WholeLine {
		return equalTrimmed(line, b.Open)
	}
	return hasPrefix(trimLeadingSpace(line), b.Open)
}

func isBlankLine(line []byte) bool {
	for _, b := range line {
		if b != ' ' && b != '\t' && b != '\r' {
			return false
		}
	}
	return true
}

func trimLeadingSpace(line []byte) []byte {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[i:]
}

func trimTrailingSpace(line []byte) []byte {
	end := len(line)
	for end > 0 {
		c := line[end-1]
		if c != ' ' && c != '\t' && c != '\r' {
			break
		}
		end--
	}
	return line[:end]
}

func hasPrefix(s []byte, p string) bool {
	return len(s) >= len(p) && string(s[:len(p)]) == p
}

func hasSuffix(s []byte, suf string) bool {
	return len(s) >= len(suf) && string(s[len(s)-len(suf):]) == suf
}

func equalTrimmed(line []byte, want string) bool {
	return string(trimTrailingSpace(trimLeadingSpace(line))) == want
}

func bytesContains(s []byte, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if string(s[i:i+len(sub)]) == sub {
			return true
		}
	}
	return false
}
