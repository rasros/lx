package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func ObjCEmitInterface(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
}
