package langs

import (
	"bytes"
	"regexp"

	"github.com/rasros/lx/pkg/lx/internal"
)

// BalanceConditionals adds to kept the directive rows of every
// preprocessor conditional (#if/#ifdef/#ifndef ... #elif/#else ... #endif)
// that holds a kept row, among its directives or between them, so kept lines
// show the conditionals around them and a kept #endif never stands without
// its #if, nor an #if without its #else and #endif. A directive spans its `\`
// continuation rows. Rows marked in hidden (inside a comment or raw string)
// hold no directives. A conditional left open at the end of the file counts
// only its directive rows.
//
// The include guard, a conditional around the whole file, is left out: it
// says nothing about what it holds.
//
// It reads the lines rather than the tree: the C and C++ grammars cannot
// nest a conditional that splits a declaration (two signatures for one body,
// an `extern "C" {` opened in one #ifdef and closed in another), and error
// recovery leaves its directives as loose tokens or invents a zero-width
// #endif.
func BalanceConditionals(lines [][]byte, kept []internal.LineRange, hidden []bool) []internal.LineRange {
	// keptBefore[row] counts the kept rows before row.
	keptBefore := make([]int, len(lines)+1)
	isKept := make([]bool, len(lines))
	for _, r := range kept {
		for row := max(r.Start, 0); row < min(r.End, len(lines)); row++ {
			isKept[row] = true
		}
	}
	for row, k := range isKept {
		keptBefore[row+1] = keptBefore[row]
		if k {
			keptBefore[row+1]++
		}
	}

	var stack, groups [][]span
	firstRow, lastRow := -1, -1 // of the first and last directive

	for row := 0; row < len(lines); row++ {
		if row < len(hidden) && hidden[row] {
			continue
		}
		from := row
		for row < len(lines)-1 && bytes.HasSuffix(bytes.TrimRight(lines[row], " \t\r"), []byte(`\`)) {
			row++
		}
		s := span{from, row}
		switch conditionalDirective(lines[from]) {
		case "if", "ifdef", "ifndef":
			stack = append(stack, []span{s})
		case "elif", "elifdef", "elifndef", "else":
			if n := len(stack); n > 0 {
				stack[n-1] = append(stack[n-1], s)
			}
		case "endif":
			if n := len(stack); n > 0 {
				groups = append(groups, append(stack[n-1], s))
				stack = stack[:n-1]
			}
		default:
			continue
		}
		if firstRow < 0 {
			firstRow = from
		}
		lastRow = row
	}
	groups = append(groups, stack...) // left open at the end of the file

	var out []internal.LineRange
	for _, g := range groups {
		first, last := g[0], g[len(g)-1]
		if keptBefore[last.to+1] == keptBefore[first.from] {
			continue
		}
		if len(g) == 2 && first.from == firstRow && last.to == lastRow && includeGuard(lines, hidden, first, last) {
			continue
		}
		for _, s := range g {
			out = internal.KeepLines(out, lines, s.from, s.to)
		}
	}
	if len(out) == 0 {
		return kept
	}
	return append(kept, out...)
}

// span is the rows of one directive, inclusive.
type span struct{ from, to int }

var guardOpen = regexp.MustCompile(`^\s*#\s*(ifndef\s+\w+|if\s+!\s*defined\s*(\(\s*\w+\s*\)|\s\w+))\s*(//.*|/\*.*)?$`)

// includeGuard reports a conditional from the directive open to the #endif
// close that opens like an include guard and has only comments and blank
// lines outside it.
func includeGuard(lines [][]byte, hidden []bool, o, c span) bool {
	if o.from != o.to || !guardOpen.Match(lines[o.from]) {
		return false
	}
	for row := range lines {
		if row >= o.from && row <= c.to || row < len(hidden) && hidden[row] {
			continue
		}
		line := bytes.TrimSpace(lines[row])
		if len(line) > 0 && !bytes.HasPrefix(line, []byte("//")) && !bytes.HasPrefix(line, []byte("/*")) {
			return false
		}
	}
	return true
}

// conditionalDirective returns the directive name of a line like `  # if
// X`, or "" for any other line.
func conditionalDirective(line []byte) string {
	line = bytes.TrimLeft(line, " \t")
	if len(line) == 0 || line[0] != '#' {
		return ""
	}
	line = bytes.TrimLeft(line[1:], " \t")
	end := 0
	for end < len(line) && line[end] >= 'a' && line[end] <= 'z' {
		end++
	}
	return string(line[:end])
}
