package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// CSharpPrivate reports a private member. Interface and enum members,
// explicit interface implementations and finalizers are public; others are
// private unless declared public, protected or internal.
func CSharpPrivate(c *Ctx, parent, n *gotreesitter.Node) bool {
	switch c.Type(parent) {
	case "interface_declaration", "enum_declaration":
		return false
	}
	if !n.IsNamed() || c.Type(n) == "destructor_declaration" {
		return false
	}
	access := ""
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch c.Type(child) {
		case "modifier":
			switch m := strings.TrimSpace(c.Text(child)); m {
			case "private":
				return true
			case "public", "protected", "internal":
				access = m
			}
		case "explicit_interface_specifier":
			return false
		}
	}
	return access == ""
}
