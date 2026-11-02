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

// CppMember classifies a C++ class member: a function_definition is a
// method, and so is a field_declaration or declaration (constructors) that
// declares a function; any other of those is a field.
func CppMember(c *Ctx, n *gotreesitter.Node) MemberKind {
	switch c.Type(n) {
	case "function_definition":
		return Method
	case "field_declaration", "declaration":
		if hasDescendantOfType(n, "function_declarator", c.Lang) {
			return Method
		}
		return Field
	}
	return Skip
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
