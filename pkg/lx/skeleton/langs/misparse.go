package langs

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// EmitMisparsed emits a C or C++ declaration the parser failed on (most
// often C++ read as C: a class taken for a declaration). Its tree says
// nothing reliable, so its rows are read as text instead:
//
//   - rows inside a function body are left out, the body's closing row too;
//     a `{` opens a function body when the code before it on its row (or the
//     row before, for a brace on a row of its own) ends in `)` or a
//     qualifier after one (`const`, `override`, ...) or in `do`, `else` or
//     `try`, or when it opens inside another function body;
//   - statement rows (`return ...;`, `if (...)`, an assignment) are left
//     out;
//   - with types alone, rows that call or declare something (a name before
//     `(`) are left out as methods; with functions alone, everything else
//     outside function bodies is kept, as the misparse cannot tell fields
//     from methods.
//
// A misparsed function definition may be a class in disguise, and it may
// run on past the function into the members after it. Until its first `{`,
// which tells a function from a class, its rows wait: a function's rows
// (its signature) are kept with functions only; a class's with either.
// Either way the rows after go on as above.
func EmitMisparsed(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	undecided := c.Type(n) == "function_definition"
	var pending []int // rows kept while undecided
	first, last := startRow(n), endRow(n)
	var stack []bool // per open brace: whether it opens a function body
	inComment := false
	prevCode := ""
	parens := 0 // open parentheses of a method row left out with types alone
	for row := first; row < len(c.Lines); row++ {
		// Past the node, only the rows closing a type it opened.
		if row > last && len(stack) == 0 {
			break
		}
		inFunc := inFuncStack(stack)
		var code string
		code, inComment = stripCode(c.Lines[row], inComment)
		text := bytes.TrimSpace(c.Lines[row])
		method := !c.Functions && !undecided && (parens > 0 || methodRow.MatchString(code))
		if method {
			parens += strings.Count(code, "(") - strings.Count(code, ")")
			parens = max(parens, 0)
		}
		keep := !inFunc && len(text) > 0 && !statementRow.Match(bytes.TrimSpace([]byte(code))) && !method
		depth := len(stack)
		function := false
		for i := 0; i < len(code); i++ {
			switch code[i] {
			case '{':
				before := bytes.TrimSpace([]byte(code[:i]))
				if len(before) == 0 {
					before = []byte(prevCode)
				}
				fn := inFuncStack(stack) || funcOpener.Match(before)
				if undecided && len(stack) == 0 {
					function = fn
				}
				stack = append(stack, fn)
			case '}':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
		if row > last && !(keep && len(stack) < depth && text[0] == '}') {
			keep = false
		}
		if keep {
			pending = append(pending, row)
		}
		if undecided && strings.Contains(code, "{") {
			undecided = false
			if function && !c.Functions {
				pending = pending[:0]
			}
		}
		if !undecided {
			for _, r := range pending {
				out = internal.KeepLine(out, c.Lines, r)
			}
			pending = pending[:0]
		}
		if t := bytes.TrimSpace([]byte(code)); len(t) > 0 {
			prevCode = string(t)
		}
	}
	if c.Functions {
		for _, r := range pending { // never opened a brace
			out = internal.KeepLine(out, c.Lines, r)
		}
	}
	return out
}

func inFuncStack(stack []bool) bool {
	for _, f := range stack {
		if f {
			return true
		}
	}
	return false
}

var (
	statementRow = regexp.MustCompile(`^(return\b|if\s*\(|for\s*\(|while\s*\(|switch\s*\(|case\b|break;|continue;|else\b|do\b|goto\b|[A-Za-z_][\w.\->\[\]]*\s*(=|\+=|-=|\*=|/=|\+\+|--)[^=])`)
	funcOpener   = regexp.MustCompile(`(\)\s*((const|override|final|noexcept|mutable|volatile|&|&&)\s*)*(->\s*[\w:<>,\s*&]+)?|\b(do|else|try))$`)
	methodRow    = regexp.MustCompile(`^\s*[^=(]*\w\s*\(`)
)

// stripCode returns line with comments and string and character literals
// blanked out, and whether a block comment is still open after it.
func stripCode(line []byte, inComment bool) (string, bool) {
	out := make([]byte, 0, len(line))
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case inComment:
			if ch == '*' && i+1 < len(line) && line[i+1] == '/' {
				inComment = false
				i++
			}
		case ch == '/' && i+1 < len(line) && line[i+1] == '/':
			return string(out), false
		case ch == '/' && i+1 < len(line) && line[i+1] == '*':
			inComment = true
			i++
		case ch == '\'' && i > 0 && isDigit(line[i-1]) && i+1 < len(line) && isHexDigit(line[i+1]):
			out = append(out, ch) // a digit separator: 10'000
		case ch == '"' || ch == '\'':
			q := ch
			for i++; i < len(line) && line[i] != q; i++ {
				if line[i] == '\\' {
					i++
				}
			}
			out = append(out, q, q)
		default:
			out = append(out, ch)
		}
	}
	return string(out), inComment
}

// DropStrayClosers drops from kept the rows that start by closing a brace
// and open none (`}`, `};`, `} name;`, `});`, `} while (0)`) whose opening
// row is not kept: they say nothing without it. Rows marked in hidden (inside comments and raw strings) hold no
// braces.
func DropStrayClosers(lines [][]byte, kept []internal.LineRange, hidden []bool) []internal.LineRange {
	isKept := make(map[int]bool)
	for _, r := range kept {
		for row := max(r.Start, 0); row < min(r.End, len(lines)); row++ {
			isKept[row] = true
		}
	}
	drop := make(map[int]bool)
	for row := range isKept {
		text := bytes.TrimSpace(lines[row])
		if len(text) == 0 || text[0] != '}' || !closerRow.Match(text) {
			continue
		}
		if opener := openingRow(lines, hidden, row); opener < 0 || !isKept[opener] {
			drop[row] = true
		}
	}
	if len(drop) == 0 {
		return kept
	}
	var out []internal.LineRange
	for _, r := range kept {
		for row := r.Start; row < r.End; row++ {
			if drop[row] {
				continue
			}
			lr := internal.LineRange{Start: row, End: row + 1}
			if row == r.End-1 {
				lr.Cut = r.Cut
			}
			out = append(out, lr)
		}
	}
	return out
}

var misparsedTypes = []string{"declaration", "function_definition", "struct_specifier", "class_specifier", "union_specifier", "type_definition"}

// Misparsed reports a declaration the parser failed on that holds a
// function declarator: a class or a function it could not read, to be
// emitted by EmitMisparsed. A function definition counts too: its body may
// be one the parser found too late, so the rows up to it are no signature.
func Misparsed(c *Ctx, n *gotreesitter.Node) bool {
	return n.HasError() && isOneOf(c.Type(n), misparsedTypes) && hasDescendantOfType(c, n, "function_declarator")
}

var misrecovered = regexp.MustCompile(`^(}|(else|return|if|while|for|do|switch|case|goto|catch|break|continue|default)\b)`)

// Misrecovered reports a node error recovery made of a statement: one that
// starts with a closing brace (`} else {`, `} catch (...) {`) or a
// statement keyword (`else if (...)`, `return f();`).
func Misrecovered(c *Ctx, n *gotreesitter.Node) bool {
	row := startRow(n)
	if row >= len(c.Lines) {
		return false
	}
	line := c.Lines[row]
	col := int(n.StartPoint().Column)
	if col > len(line) {
		return false
	}
	return misrecovered.Match(bytes.TrimSpace(line[col:]))
}

var closerRow = regexp.MustCompile(`^}[^{]*$`)

// openingRow returns the row of the `{` the first `}` on row closes, or -1.
func openingRow(lines [][]byte, hidden []bool, row int) int {
	code, _ := stripCode(lines[row], false)
	pos := bytes.IndexByte([]byte(code), '}')
	depth := 1
	seg := code[:pos]
	for r := row; r >= 0; r-- {
		if r != row {
			if r < len(hidden) && hidden[r] {
				continue
			}
			seg, _ = stripCode(lines[r], false)
		}
		for i := len(seg) - 1; i >= 0; i-- {
			switch seg[i] {
			case '}':
				depth++
			case '{':
				depth--
				if depth == 0 {
					return r
				}
			}
		}
	}
	return -1
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isHexDigit(b byte) bool {
	return isDigit(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
