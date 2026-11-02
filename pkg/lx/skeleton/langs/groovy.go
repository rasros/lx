package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// GroovyPrivate reports a member declared private.
func GroovyPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	return strings.HasPrefix(strings.TrimSpace(c.Text(n)), "private ")
}
