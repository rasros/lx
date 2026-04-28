package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func GroovyEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	body := findChildByType(n, "closure", lang)
	if body == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerEnd := int(body.StartPoint().Row)
	out = internal.AppendLines(out, lines, int(n.StartPoint().Row), headerEnd)

	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Type(lang) {
		case "declaration":
			out = EmitWithDoc(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "function_definition":
			if functions && !strings.HasPrefix(strings.TrimSpace(string(child.Text(src))), "private ") {
				out = EmitLeadingDoc(out, lines, int(child.StartPoint().Row))
				out = emitFuncSig(out, lines, child, lang, false, "closure")
			}
		}
	}

	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = internal.AppendLine(out, lines, closingRow)
	}
	return out
}
