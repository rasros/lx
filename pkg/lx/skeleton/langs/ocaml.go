package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func OCamlEmitDefinition(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	switch n.Type(lang) {
	case "type_definition":
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	case "value_definition":
		if !functions {
			return out
		}
		binding := findChildByType(n, "let_binding", lang)
		if binding == nil {
			return internal.AppendLine(out, lines, int(n.StartPoint().Row))
		}
		body := binding.ChildByFieldName("body", lang)
		if body == nil {
			return internal.AppendLine(out, lines, int(n.StartPoint().Row))
		}
		sigRow := int(n.StartPoint().Row)
		bodyRow := int(body.StartPoint().Row)
		if bodyRow > sigRow {
			return internal.AppendLines(out, lines, sigRow, bodyRow-1)
		}
		return internal.AppendLine(out, lines, sigRow)
	case "module_type_definition":
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	case "module_definition":
		return ocamlEmitModule(out, src, lines, n, lang, functions)
	}
	return out
}

func ocamlEmitModule(out, src []byte, lines [][]byte, n *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte {
	binding := findChildByType(n, "module_binding", lang)
	if binding == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	structure := findChildByType(binding, "structure", lang)
	if structure == nil {
		return internal.AppendLines(out, lines, int(n.StartPoint().Row), int(n.EndPoint().Row))
	}
	out = internal.AppendLines(out, lines, int(n.StartPoint().Row), int(structure.StartPoint().Row))
	for i := 0; i < structure.ChildCount(); i++ {
		out = OCamlEmitDefinition(out, src, lines, structure.Child(i), lang, functions)
	}
	if endRow := int(structure.EndPoint().Row); endRow > int(structure.StartPoint().Row) {
		out = internal.AppendLine(out, lines, endRow)
	}
	return out
}
