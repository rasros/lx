package langs

import (
	"github.com/odvcencio/gotreesitter"
)

func ZigFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.ChildCount() == 0 {
		return false
	}
	return n.Child(0).Type(lang) == "pub"
}
