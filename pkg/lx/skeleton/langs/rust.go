package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

func RustFuncVisible(c *Ctx, n *gotreesitter.Node) bool {
	return rustIsPublic(n, c.Src, c.Lang)
}

// RustPrivate reports a struct field or inherent impl item without pub.
// Enum variants and trait items share their parent's visibility.
func RustPrivate(c *Ctx, parent, n *gotreesitter.Node) bool {
	switch c.Type(parent) {
	case "enum_item", "trait_item":
		return false
	case "impl_item":
		if parent.ChildByFieldName("trait", c.Lang) != nil {
			return false
		}
	}
	return !rustIsPublic(n, c.Src, c.Lang)
}

func rustIsPublic(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.ChildCount() > 0 && n.Child(0).Type(lang) == "visibility_modifier" {
		return strings.HasPrefix(n.Child(0).Text(src), "pub")
	}
	return false
}
