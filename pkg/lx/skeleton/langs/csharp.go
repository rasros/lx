package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// CSharpPrivate reports a member with a private modifier.
func CSharpPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		if c.Type(child) == "modifier" && strings.TrimSpace(c.Text(child)) == "private" {
			return true
		}
	}
	return false
}
