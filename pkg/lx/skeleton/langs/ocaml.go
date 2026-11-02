package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// OCamlEmitDefinition emits an OCaml top-level or module-level definition.
// It stays custom because OCaml has no class bodies to walk: functions are
// let bindings whose signature runs up to the row before the body
// expression, and a module's members are definitions again, nested inside a
// module_binding's structure.
func OCamlEmitDefinition(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	switch c.Type(n) {
	case "type_definition", "module_type_definition":
		if c.Structs {
			return c.KeepNode(out, n)
		}
	case "value_definition":
		if !c.Functions {
			return out
		}
		sigRow := startRow(n)
		binding := findChildByType(n, "let_binding", c.Lang)
		if binding == nil {
			return internal.KeepLine(out, c.Lines, sigRow)
		}
		body := binding.ChildByFieldName("body", c.Lang)
		if body == nil {
			return internal.KeepLine(out, c.Lines, sigRow)
		}
		if bodyRow := startRow(body); bodyRow > sigRow {
			return internal.KeepLines(out, c.Lines, sigRow, bodyRow-1)
		}
		return internal.KeepLine(out, c.Lines, sigRow)
	case "module_definition":
		return ocamlEmitModule(c, out, n)
	}
	return out
}

func ocamlEmitModule(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	binding := findChildByType(n, "module_binding", c.Lang)
	var structure *gotreesitter.Node
	if binding != nil {
		structure = findChildByType(binding, "structure", c.Lang)
	}
	if structure == nil {
		if !c.Structs {
			return out
		}
		return c.KeepNode(out, n)
	}
	var inner []internal.LineRange
	for i := 0; i < structure.ChildCount(); i++ {
		child := structure.Child(i)
		if sub := OCamlEmitDefinition(c, nil, child); len(sub) > 0 {
			inner = c.EmitLeadingDoc(inner, startRow(child))
			inner = append(inner, sub...)
		}
	}
	if !c.Structs && len(inner) == 0 {
		return out
	}
	out = internal.KeepLines(out, c.Lines, startRow(n), startRow(structure))
	out = append(out, inner...)
	if end := endRow(structure); end > startRow(structure) {
		out = internal.KeepLine(out, c.Lines, end)
	}
	return out
}
