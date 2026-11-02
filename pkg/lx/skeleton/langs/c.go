package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func CFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.Type(lang) == "declaration" {
		return hasDescendantOfType(n, "function_declarator", lang)
	}
	return true
}

// CEmitTypedef emits `typedef struct { ... } Name;` (or union): the members
// are fields of the struct specifier inside the typedef, but the header and
// closing row are the typedef's own, so the name after the closing brace is
// kept. Other typedefs keep nothing. A typedef has no methods, so it is kept
// only with types.
func CEmitTypedef(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	if !c.Structs {
		return out
	}
	var specifier *gotreesitter.Node
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		t := c.Type(child)
		if t == "struct_specifier" || t == "union_specifier" {
			specifier = child
			break
		}
	}
	if specifier == nil {
		return out
	}
	fieldList := specifier.ChildByFieldName("body", c.Lang)
	if fieldList == nil {
		return out
	}
	headerEnd := startRow(fieldList)
	out = internal.KeepLines(out, c.Lines, startRow(n), headerEnd)
	for i := 0; i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(i)
		if c.Type(child) == "field_declaration" {
			out = c.EmitWithDoc(out, startRow(child), endRow(child))
		}
	}
	if closingRow := endRow(n); closingRow > headerEnd {
		out = internal.KeepLine(out, c.Lines, closingRow)
	}
	return out
}
