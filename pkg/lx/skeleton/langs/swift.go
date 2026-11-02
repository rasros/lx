package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

func SwiftFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return !SwiftPrivate(&Ctx{Src: src, Lang: lang}, nil, n)
}

// SwiftPrivate reports a private or fileprivate member.
func SwiftPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	mods := findChildByType(n, "modifiers", c.Lang)
	if mods == nil {
		return false
	}
	for i := 0; i < mods.ChildCount(); i++ {
		child := mods.Child(i)
		if c.Type(child) == "visibility_modifier" {
			text := strings.TrimSpace(c.Text(child))
			if text == "private" || text == "fileprivate" {
				return true
			}
		}
	}
	return false
}
