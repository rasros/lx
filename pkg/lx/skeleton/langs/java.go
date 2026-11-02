package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// JavaPrivate reports a member with a private modifier.
func JavaPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	if mods := findChildByType(n, "modifiers", c.Lang); mods != nil {
		for i := 0; i < mods.ChildCount(); i++ {
			if c.Type(mods.Child(i)) == "private" {
				return true
			}
		}
	}
	return strings.HasPrefix(strings.TrimSpace(c.Text(n)), "private ")
}
