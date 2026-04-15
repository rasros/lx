package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func AdaFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	return true
}

func AdaEmitDecl(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	switch n.Type(lang) {
	case "package_declaration", "subprogram_declaration":
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	return out
}
