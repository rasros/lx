package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

func ScalaFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return !ScalaPrivate(&Ctx{Src: src, Lang: lang}, nil, n)
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
