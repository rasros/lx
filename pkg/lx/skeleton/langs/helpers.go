package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func isUppercase(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

func isPrivateName(name string) bool {
	return len(name) > 0 && name[0] == '_' && (len(name) < 2 || name[1] != '_')
}

func findChildByType(n *gotreesitter.Node, typ string, lang *gotreesitter.Language) *gotreesitter.Node {
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type(lang) == typ {
			return child
		}
	}
	return nil
}

func hasDescendantOfType(n *gotreesitter.Node, typ string, lang *gotreesitter.Language) bool {
	if n.Type(lang) == typ {
		return true
	}
	for i := 0; i < n.ChildCount(); i++ {
		if hasDescendantOfType(n.Child(i), typ, lang) {
			return true
		}
	}
	return false
}

func emitFuncSig(out []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, indentBody bool, bodyChildType string) []byte {
	body := n.ChildByFieldName("body", lang)
	if body == nil && bodyChildType != "" {
		body = findChildByType(n, bodyChildType, lang)
	}
	startRow := int(n.StartPoint().Row)
	if body == nil {
		return content.AppendLines(out, lines, startRow, int(n.EndPoint().Row))
	}
	endRow := int(body.StartPoint().Row)
	if indentBody {
		endRow--
	}
	return content.AppendLines(out, lines, startRow, endRow)
}
