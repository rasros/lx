package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// ObjCUnwrap returns what an instance_variable holds: a declaration or a
// visibility section label.
func ObjCUnwrap(c *Ctx, n *gotreesitter.Node) *gotreesitter.Node {
	if c.Type(n) == "instance_variable" && n.NamedChildCount() > 0 {
		return n.NamedChild(0)
	}
	return n
}

// ObjCSectionPrivate reports an @private or @package instance variable
// section; @protected (the default) and @public ones stay visible.
func ObjCSectionPrivate(c *Ctx, label *gotreesitter.Node) bool {
	switch strings.TrimSpace(c.Text(label)) {
	case "@private", "@package":
		return true
	}
	return false
}
