package langs

import (
	"github.com/odvcencio/gotreesitter"
)

func ZigFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	start := int(n.StartByte())
	if start < 4 {
		return false
	}
	return string(src[start-4:start]) == "pub "
}
