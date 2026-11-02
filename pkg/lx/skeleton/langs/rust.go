package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

func RustFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return rustIsPublic(n, src, lang)
}

// RustPrivate reports a struct field or inherent impl item without a pub
// visibility. Enum variants, trait items and the items of a trait impl
// (`impl Trait for X`) have no visibility of their own: they are as visible
// as the enum or trait.
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
