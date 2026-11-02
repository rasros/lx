package langs

import (
	"bytes"

	"github.com/rasros/lx/pkg/lx/internal"
)

// BalanceConditionals adds to kept the directive rows of every
// preprocessor conditional (#if/#ifdef/#ifndef ... #elif/#else ... #endif)
// that holds a kept row, among its directives or between them, so kept lines
// show the conditionals around them and a kept #endif never stands without
// its #if, nor an #if without its #else and #endif. A directive spans its `\`
// continuation rows. Rows marked in literal (inside a comment or string
// literal) hold no directives.
//
// It reads the lines rather than the tree: the C and C++ grammars cannot
// nest a conditional that splits a declaration (two signatures for one body,
// an `extern "C" {` opened in one #ifdef and closed in another), and error
// recovery leaves its directives as loose tokens or invents a zero-width
// #endif.
func BalanceConditionals(lines [][]byte, kept []internal.LineRange, literal []bool) []internal.LineRange {
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

	type span struct{ from, to int } // rows of one directive, inclusive
	var stack [][]span
	var out []internal.LineRange
	closeGroup := func(g []span, end int) {
		if keptBefore[end+1] == keptBefore[g[0].from] {
			return
		}
		for _, s := range g {
			out = internal.KeepLines(out, lines, s.from, s.to)
		}
	}

	for row := 0; row < len(lines); row++ {
		if row < len(literal) && literal[row] {
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
				closeGroup(append(stack[n-1], s), row)
				stack = stack[:n-1]
			}
		}
	}
	for _, g := range stack { // unclosed at the end of the file
		closeGroup(g, len(lines)-1)
	}
	if len(out) == 0 {
		return kept
	}
	return append(kept, out...)
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
