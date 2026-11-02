package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// PHPPrivate reports a private member of a class; interface members are all
// public.
func PHPPrivate(c *Ctx, parent, n *gotreesitter.Node) bool {
	if c.Type(parent) == "interface_declaration" {
		return false
	}
	vis := findChildByType(n, "visibility_modifier", c.Lang)
	return vis != nil && strings.TrimSpace(c.Text(vis)) == "private"
}
