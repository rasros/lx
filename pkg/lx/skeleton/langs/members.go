package langs

import (
	"slices"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

// MemberKind is what a member of a type body is to the skeleton.
type MemberKind int

const (
	// Skip is a member the skeleton leaves out (statements, punctuation,
	// comments, which come back as docs).
	Skip MemberKind = iota
	// Field is kept whole with its doc when types are kept.
	Field
	// Method keeps its signature and doc when functions are kept.
	Method
	// Nested is a type declared inside the body, walked with the same rules.
	Nested
)

// Members describes the members of a language's type declarations (classes,
// interfaces, structs, traits, impls) so that one walker can emit them:
//
//   - the header: the rows from the declaration's start to its body's start
//   - each visible member: fields when types are kept, method signatures when
//     functions are kept, nested types recursively, each with its doc block
//     and any annotation siblings right before it
//   - the body's closing row
//
// With functions alone, a type that keeps no method (directly or in a nested
// type) is left out entirely; one that does keeps its header and closing row
// for context, but no fields.
type Members struct {
	// BodyField is the field name of the body node, tried first.
	BodyField string
	// Bodies are the node types of the body, the first child of one of them
	// in this order. A declaration without a body is kept whole as a type,
	// unless SkipBodiless is set (C++ forward declarations).
	Bodies       []string
	SkipBodiless bool
	// BodyIsNode is for grammars whose members are children of the
	// declaration itself (Objective-C @interface ... @end): the header is the
	// declaration's first row and the closing row its last.
	BodyIsNode bool

	Fields  []string
	Methods []string
	Nested  []string
	// Kind, when set, classifies members instead of the three lists, for
	// grammars where the node type alone does not tell (a C++
	// field_declaration that declares a function is a method).
	Kind func(c *Ctx, n *gotreesitter.Node) MemberKind

	// Unwrap, when set, returns the declaration a member wraps (a C++
	// template_declaration's function or class, the struct specifier of a
	// C++ `struct Inner { ... };` member, what an Objective-C instance_variable
	// holds), or the member itself. Section labels and groups are recognized
	// after unwrapping. The wrapper's rows before the declaration are kept
	// with it like annotations.
	Unwrap func(c *Ctx, n *gotreesitter.Node) *gotreesitter.Node
	// Groups are member node types whose children are members of the same
	// body: Java's enum_body_declarations after the enum constants,
	// Objective-C's instance variable block and @optional sections, and
	// preprocessor conditionals (#if ... #endif).
	Groups []string
	// Frames are groups whose delimiter rows are kept around the members
	// kept inside them, so the output stays well-formed: the first row (#if,
	// #else, @optional, "{") and, when the group ends in a token, the last
	// (#endif, "}"). A frame that opens on the header's row is always kept
	// whole.
	Frames []string
	// SectionScopes are groups that end the access sections opened inside
	// them (Objective-C's instance variable block: @private there says
	// nothing about the methods after it).
	SectionScopes []string

	// MethodBodies are the child types of a method's body when the grammar
	// has no "body" field for it (Kotlin's function_body, Groovy's closure),
	// tried in order.
	MethodBodies []string
	// Annotations are sibling node types that belong to the member after
	// them (TypeScript decorators, Dart annotations).
	Annotations []string

	// Private reports a member of parent hidden from the skeleton.
	Private func(c *Ctx, parent, n *gotreesitter.Node) bool
	// Section is the node type of an access section label (C++
	// "public:"): it sets the visibility of the members after it, as
	// SectionPrivate reports, and its row is kept when a member after it is.
	// Members of a type in PrivateByDefault start out private.
	Section          string
	SectionPrivate   func(c *Ctx, label *gotreesitter.Node) bool
	PrivateByDefault []string
}

// Emit appends the skeleton of the type declaration n.
func (m *Members) Emit(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	if m.BodyIsNode {
		return m.Walk(c, out, n, n, n)
	}
	body := m.body(c, n)
	if body == nil {
		if m.SkipBodiless || !c.Structs {
			return out
		}
		return c.KeepNode(out, n)
	}
	return m.Walk(c, out, n, body, body)
}

func (m *Members) body(c *Ctx, n *gotreesitter.Node) *gotreesitter.Node {
	if m.BodyField != "" {
		if b := n.ChildByFieldName(m.BodyField, c.Lang); b != nil {
			return b
		}
	}
	for _, t := range m.Bodies {
		if b := findChildByType(n, t, c.Lang); b != nil {
			return b
		}
	}
	return nil
}

// Walk appends the skeleton of the type n whose header runs to the first row
// of body and whose members are the children of items (usually body itself).
func (m *Members) Walk(c *Ctx, out []internal.LineRange, n, body, items *gotreesitter.Node) []internal.LineRange {
	headerEnd := startRow(body)
	w := memberWalk{m: m, c: c, parent: n, headerEnd: headerEnd, label: -1,
		private: slices.Contains(m.PrivateByDefault, c.Type(n))}
	w.walk(items)
	if !c.Structs && w.kept == 0 {
		return out
	}

	out = internal.KeepLines(out, c.Lines, startRow(n), headerEnd)
	out = append(out, w.out...)
	if closingRow := endRow(body); closingRow > headerEnd {
		out = internal.KeepLine(out, c.Lines, closingRow)
	}
	return out
}

// memberWalk is one pass over the members of a type body.
type memberWalk struct {
	m         *Members
	c         *Ctx
	parent    *gotreesitter.Node
	headerEnd int
	private   bool // inside a private access section
	label     int  // row of the access section label no member has been kept under yet, or -1
	kept      int  // members kept
	out       []internal.LineRange
}

func (w *memberWalk) walk(items *gotreesitter.Node) {
	m, c := w.m, w.c
	for i := 0; i < items.ChildCount(); i++ {
		child := items.Child(i)
		decl := child
		if m.Unwrap != nil {
			decl = m.Unwrap(c, child)
		}
		t := c.Type(decl)
		if m.Section != "" && t == m.Section {
			w.private = m.SectionPrivate(c, decl)
			w.label = startRow(decl)
			continue
		}
		if slices.Contains(m.Groups, t) {
			w.group(decl, t)
			continue
		}
		if w.private || (m.Private != nil && m.Private(c, w.parent, child)) {
			continue
		}
		from := startRow(child)
		if len(m.Annotations) > 0 {
			from = precedingSiblingsRow(items, child, i, m.Annotations, c.Lang)
		}
		switch m.kind(c, decl) {
		case Field:
			if c.Structs {
				w.keep()
				w.out = c.EmitWithDoc(w.out, from, endRow(child))
			}
		case Method:
			if c.Functions {
				w.keep()
				w.out = c.EmitLeadingDoc(w.out, from)
				w.out = EmitDecorators(w.out, c.Lines, from, startRow(decl))
				w.out = EmitFuncSig(w.out, c.Lines, decl, c.Lang, false, m.methodBody(c, decl))
			}
		case Nested:
			if sub := m.Emit(c, nil, decl); len(sub) > 0 {
				w.keep()
				w.out = c.EmitLeadingDoc(w.out, from)
				w.out = EmitDecorators(w.out, c.Lines, from, startRow(decl))
				w.out = append(w.out, sub...)
			}
		}
	}
}

// keep counts a kept member, keeping the label of its access section first.
func (w *memberWalk) keep() {
	w.kept++
	if w.label >= 0 {
		w.out = internal.KeepLine(w.out, w.c.Lines, w.label)
		w.label = -1
	}
}

// group walks the members inside g, a node of group type t, and keeps its
// frame rows around them.
func (w *memberWalk) group(g *gotreesitter.Node, t string) {
	m, c := w.m, w.c
	private, label, outer, keptBefore := w.private, w.label, w.out, w.kept
	w.out = nil
	w.walk(g)
	inner := w.out
	w.out = outer
	if slices.Contains(m.SectionScopes, t) {
		w.private, w.label = private, label
	}
	first, last := startRow(g), endRow(g)
	if slices.Contains(m.Frames, t) && (w.kept > keptBefore || first <= w.headerEnd) {
		w.out = internal.KeepLine(w.out, c.Lines, first)
		if n := g.ChildCount(); n > 0 && !g.Child(n-1).IsNamed() && last > first {
			w.out = internal.KeepLine(w.out, c.Lines, last)
		}
	}
	w.out = append(w.out, inner...)
}

// methodBody returns the first of MethodBodies that n has a child of.
func (m *Members) methodBody(c *Ctx, n *gotreesitter.Node) string {
	for _, t := range m.MethodBodies {
		if findChildByType(n, t, c.Lang) != nil {
			return t
		}
	}
	return ""
}

func (m *Members) kind(c *Ctx, n *gotreesitter.Node) MemberKind {
	if m.Kind != nil {
		return m.Kind(c, n)
	}
	t := c.Type(n)
	switch {
	case slices.Contains(m.Fields, t):
		return Field
	case slices.Contains(m.Methods, t):
		return Method
	case slices.Contains(m.Nested, t):
		return Nested
	}
	return Skip
}
