package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

func ScalaFuncVisible(c *Ctx, n *gotreesitter.Node) bool {
	return !ScalaPrivate(c, nil, n)
}

// ScalaPrivate reports a member whose access modifier is private or
// private[scope].
func ScalaPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	mods := findChildByType(n, "modifiers", c.Lang)
	if mods == nil {
		return false
	}
	for i := 0; i < mods.ChildCount(); i++ {
		child := mods.Child(i)
		if c.Type(child) == "access_modifier" {
			return strings.HasPrefix(strings.TrimSpace(c.Text(child)), "private")
		}
	}
	return false
}
