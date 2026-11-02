package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func CppFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	if n.Type(lang) == "declaration" {
		return hasDescendantOfType(n, "function_declarator", lang)
	}
	return true
}

var cppTypeSpecifiers = []string{"class_specifier", "struct_specifier", "union_specifier", "enum_specifier"}

// CppUnwrap returns the declaration inside a template_declaration, and the
// type specifier of a member that only declares a type (`struct Inner { ...
// };`, with no declarator after the body). Anything else is returned as is.
func CppUnwrap(c *Ctx, n *gotreesitter.Node) *gotreesitter.Node {
	switch c.Type(n) {
	case "template_declaration":
		for i := n.ChildCount() - 1; i >= 0; i-- {
			if child := n.Child(i); child.IsNamed() && c.Type(child) != "template_parameter_list" {
				return CppUnwrap(c, child)
			}
		}
	case "field_declaration", "declaration":
		typ := n.ChildByFieldName("type", c.Lang)
		if typ != nil && n.ChildByFieldName("declarator", c.Lang) == nil && isOneOf(c.Type(typ), cppTypeSpecifiers) &&
			typ.ChildByFieldName("body", c.Lang) != nil {
			return typ
		}
	}
	return n
}

// CppMember classifies a C++ class member (after CppUnwrap): a
// function_definition, or a declaration whose declarator declares a
// function, is a method; a class, struct, union or enum is a nested type;
// other declarations, including function pointers, and enumerators are
// fields.
func CppMember(c *Ctx, n *gotreesitter.Node) MemberKind {
	switch t := c.Type(n); {
	case t == "function_definition":
		return Method
	case isOneOf(t, cppTypeSpecifiers):
		return Nested
	case t == "field_declaration" || t == "declaration":
		if cppDeclaresFunction(c, n.ChildByFieldName("declarator", c.Lang)) {
			return Method
		}
		return Field
	case t == "enumerator":
		return Field
	}
	return Skip
}

// cppDeclaresFunction reports a declarator that declares a function (maybe
// returning a pointer or reference), not a pointer to one: in `void
// (*callback)(int)` the function declarator's own declarator is
// parenthesized.
func cppDeclaresFunction(c *Ctx, d *gotreesitter.Node) bool {
	for d != nil {
		switch c.Type(d) {
		case "pointer_declarator", "reference_declarator":
			next := d.ChildByFieldName("declarator", c.Lang)
			if next == nil && d.NamedChildCount() > 0 {
				next = d.NamedChild(d.NamedChildCount() - 1)
			}
			d = next
		case "function_declarator":
			inner := d.ChildByFieldName("declarator", c.Lang)
			return inner == nil || c.Type(inner) != "parenthesized_declarator"
		default:
			return false
		}
	}
	return false
}

// CppSectionPrivate reports a private: section label. The grammar keeps the
// colon out of the access_specifier node.
func CppSectionPrivate(c *Ctx, label *gotreesitter.Node) bool {
	return strings.TrimSuffix(strings.TrimSpace(c.Text(label)), ":") == "private"
}

// CppEmitter emits C++ type declarations: typedefs as in C, the rest with
// the generic member walker.
func CppEmitter(m *Members) Emitter {
	return func(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
		if c.Type(n) == "type_definition" {
			return CEmitTypedef(c, out, n)
		}
		return m.Emit(c, out, n)
	}
}

func isOneOf(s string, set []string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}
