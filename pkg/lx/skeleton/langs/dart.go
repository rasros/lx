package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func DartEmitClass(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	body := n.ChildByFieldName("body", lang)
	if body == nil {
		return content.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	headerEnd := int(body.StartPoint().Row)
	out = content.AppendLines(out, lines, int(n.StartPoint().Row), headerEnd)

	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if dartMemberIsPrivate(lines, child) {
			continue
		}
		switch child.Type(lang) {
		case "declaration":
			out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
		case "method_signature":
			if functions {
				out = content.AppendLines(out, lines, int(child.StartPoint().Row), int(child.EndPoint().Row))
			}
		}
	}

	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = content.AppendLine(out, lines, closingRow)
	}
	return out
}

// dartMemberIsPrivate reports library-private members via "_" prefix.
func dartMemberIsPrivate(lines [][]byte, n *gotreesitter.Node) bool {
	row := int(n.StartPoint().Row)
	if row >= len(lines) {
		return false
	}
	line := strings.TrimSpace(string(lines[row]))
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if idx := strings.IndexAny(line, "(;{="); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	return isPrivateName(parts[len(parts)-1])
}
