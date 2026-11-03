package langs

import (
	"bytes"
	"regexp"

	"github.com/odvcencio/gotreesitter"
)

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
