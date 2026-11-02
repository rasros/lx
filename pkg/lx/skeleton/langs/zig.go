package langs

import (
	"github.com/odvcencio/gotreesitter"
)

func ZigFuncVisible(c *Ctx, n *gotreesitter.Node) bool {
	if n.ChildCount() == 0 {
		return false
	}
	return c.Type(n.Child(0)) == "pub"
}
