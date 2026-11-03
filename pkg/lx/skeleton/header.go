package skeleton

import (
	"bytes"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/rasros/lx/pkg/lx/skeleton/langs"
)

// extractHeader is extract for a .h file, which may be C or C++. It is read
// as C++ when the C++ grammar parses it without errors and it holds a
// construct only C++ has (see cppEvidence), and as C otherwise. A header
// the C++ grammar fails on stays C, as before: that grammar loses class
// and namespace headers on some templates and macros, so a C++ tree with
// errors can show less than the C one.
//
// The C++ grammar goes first when the text hints at C++ (see cppHint), so
// most C++ headers are parsed once, and a C header only when the C grammar
// fails on it.
func extractHeader(src []byte, lines [][]byte, functions, structs bool) ([]internal.LineRange, bool) {
	var cpp parsed
	if cppHint(src) {
		if p, ok := parse("cpp", src); ok {
			cpp = p
			defer cpp.tree.Release()
			if cppClean(cpp, src) {
				return cpp.walk(src, lines, functions, structs), true
			}
		}
	}
	c, ok := parse("c", src)
	if !ok {
		return nil, false
	}
	defer c.tree.Release()
	if c.tree.RootNode().HasError() && cpp.tree == nil {
		if p, ok := parse("cpp", src); ok {
			defer p.tree.Release()
			if cppClean(p, src) {
				return p.walk(src, lines, functions, structs), true
			}
		}
	}
	return c.walk(src, lines, functions, structs), true
}

// cppClean reports a C++ tree without errors over the whole source that
// holds C++.
func cppClean(p parsed, src []byte) bool {
	root := p.tree.RootNode()
	return !root.HasError() && int(root.EndByte()) >= len(bytes.TrimRight(src, " \t\r\n")) && cppEvidence(p, src)
}

// cppHint reports text C++ source is likely to hold and C source is not.
// It only orders the parses: either grammar may still read the header.
func cppHint(src []byte) bool {
	for _, s := range cppHints {
		if bytes.Contains(src, s) {
			return true
		}
	}
	return false
}

var cppHints = [][]byte{
	[]byte("::"), []byte("class "), []byte("namespace "), []byte("template"), []byte("public:"), []byte("operator"),
}

// cppOnly are node types of constructs C does not have.
var cppOnly = map[string]bool{
	"namespace_definition":          true,
	"class_specifier":               true,
	"template_declaration":          true,
	"template_instantiation":        true,
	"access_specifier":              true,
	"using_declaration":             true,
	"alias_declaration":             true,
	"qualified_identifier":          true,
	"reference_declarator":          true,
	"abstract_reference_declarator": true,
	"operator_name":                 true,
	"destructor_name":               true,
	"friend_declaration":            true,
	"base_class_clause":             true,
	"field_initializer_list":        true,
	"virtual_specifier":             true,
	"explicit_function_specifier":   true,
	"template_type":                 true,
	"template_function":             true,
	"raw_string_literal":            true,
	"new_expression":                true,
	"delete_expression":             true,
	"lambda_expression":             true,
}

// cppEvidence reports a construct only C++ has that the C++ grammar parsed
// whole, outside function bodies, in the tree p: a class or namespace, a
// template, a reference, an access label, a method, a scoped enum, a digit
// separator and the like. Attributes (`[[gnu::x]]`) and inline asm clobber
// lists, which look like C++ to a text scan, are C too.
func cppEvidence(p parsed, src []byte) bool {
	var walk func(n *gotreesitter.Node, inBody bool) bool
	walk = func(n *gotreesitter.Node, inBody bool) bool {
		// The children of a node with errors may still be whole.
		if !n.HasError() && cppOnlyNode(p, n, src, inBody) {
			return true
		}
		t := n.Type(p.lang)
		if t == "compound_statement" {
			return false
		}
		body := t == "field_declaration_list"
		for i := 0; i < n.ChildCount(); i++ {
			if walk(n.Child(i), body) {
				return true
			}
		}
		return false
	}
	return walk(p.tree.RootNode(), false)
}

// cppOnlyNode reports whether the well-formed node n is C++ only: see
// cppEvidence. inBody is set for the members of a class or struct body.
func cppOnlyNode(p parsed, n *gotreesitter.Node, src []byte, inBody bool) bool {
	t := n.Type(p.lang)
	switch {
	case cppOnly[t]:
		return true
	case inBody && (t == "function_definition" || t == "field_declaration"):
		c := &langs.Ctx{Src: src, Lang: p.lang}
		return langs.CppMember(c, n) == langs.Method
	case t == "enum_specifier":
		for i := 0; i < n.ChildCount(); i++ {
			if ct := n.Child(i).Type(p.lang); ct == "class" || ct == "struct" {
				return true
			}
		}
	case t == "number_literal":
		return bytes.IndexByte(src[n.StartByte():n.EndByte()], '\'') >= 0
	}
	return false
}
