package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func ObjCEmitInterface(out []internal.LineRange, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []internal.LineRange {
	return internal.KeepLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
}
