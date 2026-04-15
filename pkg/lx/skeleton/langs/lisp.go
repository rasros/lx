package langs

import (
	"github.com/odvcencio/gotreesitter"
)

func CLFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return findChildByType(n, "defun", lang) != nil
}
