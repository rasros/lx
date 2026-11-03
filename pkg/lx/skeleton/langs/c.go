package langs

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// CFuncVisible reports a C or C++ function definition, or a declaration
// with a declarator that declares a function. A pointer to a function
// (`int (*handler)(int);`) is a variable, not a function. A declaration
// that failed to parse never gets here (see Misparsed).
func CFuncVisible(c *Ctx, n *gotreesitter.Node) bool {
	if c.Type(n) != "declaration" {
		return true
	}
	for i := 0; i < n.ChildCount(); i++ {
		if n.FieldNameForChild(i, c.Lang) == "declarator" && declaresFunction(c, n.Child(i)) {
			return true
		}
	}
	return false
}

func hasDescendantOfType(c *Ctx, n *gotreesitter.Node, typ string) bool {
	if c.Type(n) == typ {
		return true
	}
	for i := 0; i < n.ChildCount(); i++ {
		if hasDescendantOfType(c, n.Child(i), typ) {
			return true
		}
	}
	return false
}

// declaresFunction reports a C or C++ declarator that declares a function
// (maybe returning a pointer or reference), not a pointer to or array of
// them: what decides is the declarator nearest the name. In `void
// (*callback)(int)` that is the pointer, in `int (*get(void))(int)` the
// function. An initializer is looked through: C++ read as C makes one of a
// pure virtual's `= 0` (`char const * name() const = 0;`), and
// `int (*fp)(int) = f;` stays a pointer.
func declaresFunction(c *Ctx, d *gotreesitter.Node) bool {
	function := false
	for d != nil {
		switch c.Type(d) {
		case "function_declarator":
			function = true
		case "pointer_declarator", "reference_declarator", "array_declarator":
			function = false
		case "parenthesized_declarator", "init_declarator":
		case "operator_cast": // operator int() const
			return true
		default: // the name
			return function
		}
		next := d.ChildByFieldName("declarator", c.Lang)
		if next == nil && d.NamedChildCount() > 0 {
			next = d.NamedChild(d.NamedChildCount() - 1)
		}
		d = next
	}
	return function
}

// cEmitTypedef emits a typedef, kept only with types since it has no
// methods. Of `typedef struct { ... } Name;` (or union, or enum) the members
// are walked: they belong to the specifier inside the typedef, but the header
// and closing row are the typedef's own, so the name after the closing brace
// is kept. Any other typedef is a type alias, kept whole.
func cEmitTypedef(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	if !c.Structs {
		return out
	}
	var body *gotreesitter.Node
	if typ := n.ChildByFieldName("type", c.Lang); typ != nil && isOneOf(c.Type(typ), cTypeSpecifiers) {
		body = typ.ChildByFieldName("body", c.Lang)
	}
	if body == nil {
		return c.KeepNode(out, n)
	}
	headerEnd := startRow(body)
	out = internal.KeepLines(out, c.Lines, startRow(n), headerEnd)
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		if t := c.Type(child); t == "field_declaration" || t == "enumerator" {
			out = c.EmitWithDoc(out, startRow(child), endRow(child))
		}
	}
	if closingRow := endRow(n); closingRow > headerEnd {
		out = internal.KeepLine(out, c.Lines, closingRow)
	}
	return out
}

var cTypeSpecifiers = []string{"struct_specifier", "union_specifier", "enum_specifier"}
