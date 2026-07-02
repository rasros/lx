package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// LeadingDocStart returns the first row of a contiguous comment block
// immediately preceding row, or row itself if there is none. A blank line
// terminates the block.
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
		closer, opener := blockCommentClosers(line)
		if closer == "" {
			break
		}
		found := false
		for r >= 0 {
			if hasOpener(lines[r], opener) {
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

// EmitWithDoc appends [LeadingDocStart(lines, startRow) .. endRow] to out.
func EmitWithDoc(out []byte, lines [][]byte, startRow, endRow int) []byte {
	return internal.AppendLines(out, lines, LeadingDocStart(lines, startRow), endRow)
}

// EmitLineWithDoc appends the leading doc block plus row itself.
func EmitLineWithDoc(out []byte, lines [][]byte, row int) []byte {
	return EmitWithDoc(out, lines, row, row)
}

// EmitLeadingDoc appends only the leading doc block above row, if any.
func EmitLeadingDoc(out []byte, lines [][]byte, row int) []byte {
	docStart := LeadingDocStart(lines, row)
	if docStart < row {
		return internal.AppendLines(out, lines, docStart, row-1)
	}
	return out
}

// AppendPyDocstring appends the Python docstring (if any) of n's body to out.
func AppendPyDocstring(out []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language) []byte {
	start, end := PyDocstring(n, lang)
	if start < 0 {
		return out
	}
	return internal.AppendLines(out, lines, start, end)
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
	case s[0] == '*':
		return true
	case s[0] == '#':
		return true
	case hasPrefix(s, "--"):
		return true
	case hasPrefix(s, "{-"), hasPrefix(s, "-}"):
		return true
	case hasPrefix(s, "(*"), hasPrefix(s, "*)"):
		return true
	case hasPrefix(s, "=begin"):
		// A lone "=begin" line is the opener of a Ruby block comment; the
		// "=end" closer is handled by blockCommentClosers so the whole block
		// is walked as a unit. (=end must NOT match here, or the block scan
		// would consume it and stop before reaching =begin.)
		return true
	}
	return false
}

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
		// Self-contained inline (e.g. `int x; /* note */`) is not a multi-line closer.
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

func isBlankLine(line []byte) bool {
	for _, b := range line {
		if b != ' ' && b != '\t' && b != '\r' {
			return false
		}
	}
	return true
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

func bytesContains(s []byte, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
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
