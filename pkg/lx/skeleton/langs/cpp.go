package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

var cppTypeSpecifiers = []string{"class_specifier", "struct_specifier", "union_specifier", "enum_specifier"}

// CppUnwrap returns the declaration inside a template_declaration, the
// function a friend declaration declares, the one declaration of an
// `extern "C"` without braces, and the type specifier of a member that only
// declares a type (`struct Inner { ... };`, with no declarator after the
// body). Anything else is returned as is. It serves C too.
func CppUnwrap(c *Ctx, n *gotreesitter.Node) *gotreesitter.Node {
	switch c.Type(n) {
	case "linkage_specification":
		if body := n.ChildByFieldName("body", c.Lang); body != nil && c.Type(body) != "declaration_list" {
			return CppUnwrap(c, body)
		}
	case "friend_declaration":
		for i := 0; i < n.NamedChildCount(); i++ {
			inner := n.NamedChild(i)
			switch c.Type(inner) {
			case "function_definition":
				return inner
			case "declaration":
				if declaresFunction(c, inner.ChildByFieldName("declarator", c.Lang)) {
					return inner
				}
			}
		}
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

// CppMember classifies a C++ class member (after CppUnwrap): functions are
// methods, classes, structs, unions and enums nested types, and anything
// else a field.
func CppMember(c *Ctx, n *gotreesitter.Node) MemberKind {
	switch t := c.Type(n); {
	case t == "function_definition":
		return Method
	case isOneOf(t, cppTypeSpecifiers):
		return Nested
	case t == "field_declaration" || t == "declaration":
		if declaresFunction(c, n.ChildByFieldName("declarator", c.Lang)) {
			return Method
		}
		return Field
	case t == "enumerator" || t == "friend_declaration" || t == "type_definition" || t == "alias_declaration":
		return Field
	}
	return Skip
}

// CppSectionPrivate reports a private: section label. The grammar keeps the
// colon out of the access_specifier node.
func CppSectionPrivate(c *Ctx, label *gotreesitter.Node) bool {
	return strings.TrimSuffix(strings.TrimSpace(c.Text(label)), ":") == "private"
}

// CppEmitter emits top-level C and C++ type declarations: typedefs, aliases
// and forward declarations whole with types, and the rest with the generic
// member walker.
func CppEmitter(m *Members) Emitter {
	return func(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
		switch t := c.Type(n); {
		case t == "type_definition":
			return cEmitTypedef(c, out, n)
		case t == "declaration":
			typ := n.ChildByFieldName("type", c.Lang)
			if typ == nil || !isOneOf(c.Type(typ), cppTypeSpecifiers) || typ.ChildByFieldName("body", c.Lang) == nil {
				return out
			}
			return m.Emit(c, out, typ)
		case t == "alias_declaration",
			isOneOf(t, cppTypeSpecifiers) && n.ChildByFieldName("body", c.Lang) == nil:
			if !c.Structs {
				return out
			}
			return c.KeepNode(out, n)
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
