package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// LeadingDocStart returns the first row of a leading doc-comment block
// immediately preceding `row`, or `row` itself if there is none. A doc-comment
// block is a contiguous run of comment lines (no blank line between them and
// the declaration). Recognises C-family `//`, `/* */`, ` * ...`, `///`, `//!`,
// shell/Python/Ruby `#`, Haskell `-- {- -}`, OCaml `(* *)`, and Ruby
// `=begin/=end` styles, including multi-line block comments whose continuation
// lines carry no marker.
func LeadingDocStart(lines [][]byte, row int) int {
	r := row - 1
	for r >= 0 {
		line := lines[r]
		if isBlankLine(line) {
			break
		}
		if isDocCommentLine(line) {
			r--
			continue
		}
		// Possible end of a multi-line block comment whose continuation
		// lines do not start with a recognized marker (e.g. C `/* x\n y */`).
		closer, opener := blockCommentClosers(line)
		if closer == "" {
			break
		}
		// Walk back through the block until we find the opening line.
		found := false
		for r >= 0 {
			ln := lines[r]
			if hasOpener(ln, opener) {
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

func isBlankLine(line []byte) bool {
	for _, b := range line {
		if b != ' ' && b != '\t' && b != '\r' {
			return false
		}
	}
	return true
}

// blockCommentClosers returns (closer, opener) markers if line is the closing
// line of a multi-line block comment whose opener lives on a previous line.
// Returns "", "" if the line is code, a self-contained inline comment, or has
// no closer.
func blockCommentClosers(line []byte) (string, string) {
	end := trimTrailingSpace(line)
	for _, p := range [...]struct{ closer, opener string }{
		{"*/", "/*"},
		{"*)", "(*"},
		{"-}", "{-"},
	} {
		if !hasSuffix(end, p.closer) {
			continue
		}
		// Self-contained block on this line (e.g. `int x; /* note */`)
		// is not a multi-line closer.
		if bytesContains(end[:len(end)-len(p.closer)], p.opener) {
			return "", ""
		}
		return p.closer, p.opener
	}
	if equalTrimmed(line, "=end") {
		return "=end", "=begin"
	}
	return "", ""
}

func bytesContains(s []byte, sub string) bool {
	if len(sub) == 0 || len(s) < len(sub) {
		return len(sub) == 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func hasOpener(line []byte, opener string) bool {
	if opener == "=begin" {
		return equalTrimmed(line, "=begin")
	}
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return hasPrefix(line[i:], opener)
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

func hasSuffix(s []byte, suf string) bool {
	if len(s) < len(suf) {
		return false
	}
	off := len(s) - len(suf)
	for i := 0; i < len(suf); i++ {
		if s[off+i] != suf[i] {
			return false
		}
	}
	return true
}

func equalTrimmed(line []byte, want string) bool {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	end := len(line)
	for end > i {
		c := line[end-1]
		if c != ' ' && c != '\t' && c != '\r' {
			break
		}
		end--
	}
	if end-i != len(want) {
		return false
	}
	for k := 0; k < len(want); k++ {
		if line[i+k] != want[k] {
			return false
		}
	}
	return true
}

// EmitWithDoc appends [LeadingDocStart(lines, startRow) .. endRow] to out.
func EmitWithDoc(out []byte, lines [][]byte, startRow, endRow int) []byte {
	return internal.AppendLines(out, lines, LeadingDocStart(lines, startRow), endRow)
}

// EmitLineWithDoc appends the leading doc block plus `row` itself.
func EmitLineWithDoc(out []byte, lines [][]byte, row int) []byte {
	return EmitWithDoc(out, lines, row, row)
}

// EmitLeadingDoc appends only the leading doc block above `row` (if any).
// The line at `row` itself is not appended.
func EmitLeadingDoc(out []byte, lines [][]byte, row int) []byte {
	docStart := LeadingDocStart(lines, row)
	if docStart < row {
		return internal.AppendLines(out, lines, docStart, row-1)
	}
	return out
}

func isDocCommentLine(line []byte) bool {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i == len(line) {
		return false
	}
	s := line[i:]
	switch {
	case hasPrefix(s, "///"), hasPrefix(s, "//!"), hasPrefix(s, "//"):
		return true
	case hasPrefix(s, "/**"), hasPrefix(s, "/*!"), hasPrefix(s, "/*"):
		return true
	case hasPrefix(s, "*/"):
		return true
	case len(s) > 0 && s[0] == '*':
		return true
	case len(s) > 0 && s[0] == '#':
		return true
	case hasPrefix(s, "--"):
		return true
	case hasPrefix(s, "{-"), hasPrefix(s, "-}"):
		return true
	case hasPrefix(s, "(*"), hasPrefix(s, "*)"):
		return true
	case hasPrefix(s, "=begin"), hasPrefix(s, "=end"):
		return true
	}
	return false
}

func hasPrefix(s []byte, p string) bool {
	if len(s) < len(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if s[i] != p[i] {
			return false
		}
	}
	return true
}

// AppendPyDocstring appends the Python docstring (if any) of n's body to out.
func AppendPyDocstring(out []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	start, end := PyDocstring(n, lang)
	if start < 0 {
		return out
	}
	return internal.AppendLines(out, lines, start, end)
}

// PyDocstring returns the row range [start, end] of a Python docstring inside
// the given function/class body node, or (-1, -1) if there is none. A
// docstring is the first named statement in the body and is a string literal
// expression.
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
