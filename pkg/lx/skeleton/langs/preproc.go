package langs

import (
	"bytes"

	"github.com/rasros/lx/pkg/lx/internal"
)

// BalanceConditionals adds to kept the rows of every preprocessor
// conditional (#if/#ifdef/#ifndef ... #elif/#else ... #endif) one of whose
// directive rows is kept, so a kept #endif never stands without its #if, nor
// an #if without its #else and #endif. A directive spans its `\`
// continuation rows.
//
// It reads the lines rather than the tree: the C and C++ grammars cannot
// nest a conditional that splits a declaration (two signatures for one body,
// an `extern "C" {` opened in one #ifdef and closed in another), and error
// recovery leaves its directives as loose tokens or invents a zero-width
// #endif.
func BalanceConditionals(lines [][]byte, kept []internal.LineRange) []internal.LineRange {
	isKept := make([]bool, len(lines))
	for _, r := range kept {
		for row := max(r.Start, 0); row < min(r.End, len(lines)); row++ {
			isKept[row] = true
		}
	}

	type span struct{ from, to int } // rows of one directive, inclusive
	var stack [][]span
	var out []internal.LineRange
	closeGroup := func(g []span) {
		for _, s := range g {
			for row := s.from; row <= s.to; row++ {
				if isKept[row] {
					for _, s := range g {
						out = internal.KeepLines(out, lines, s.from, s.to)
					}
					return
				}
			}
		}
	}

	for row := 0; row < len(lines); row++ {
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
				closeGroup(append(stack[n-1], s))
				stack = stack[:n-1]
			}
		}
	}
	for _, g := range stack { // unclosed at the end of the file
		closeGroup(g)
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
