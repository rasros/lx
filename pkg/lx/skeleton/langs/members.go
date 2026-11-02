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

	Fields  []string
	Methods []string
	Nested  []string
	// Kind, when set, classifies members instead of the three lists, for
	// grammars where the node type alone does not tell (a C++
	// field_declaration that declares a function is a method).
	Kind func(c *Ctx, n *gotreesitter.Node) MemberKind

	// MethodBody is the child type of a method's body when the grammar has
	// no "body" field for it (Kotlin's function_body, Groovy's closure).
	MethodBody string
	// Annotations are sibling node types that belong to the member after
	// them (TypeScript decorators, Dart annotations).
	Annotations []string

	// Private reports a member of parent hidden from the skeleton.
	Private func(c *Ctx, parent, n *gotreesitter.Node) bool
	// Section is the node type of an access section label (C++
	// "public:"): it sets the visibility of the members after it, as
	// SectionPrivate reports. Members of a type in PrivateByDefault start
	// out private.
	Section          string
	SectionPrivate   func(c *Ctx, label *gotreesitter.Node) bool
	PrivateByDefault []string
}

// Emit appends the skeleton of the type declaration n.
func (m *Members) Emit(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
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
	private := slices.Contains(m.PrivateByDefault, c.Type(n))

	var inner []internal.LineRange
	for i := 0; i < items.ChildCount(); i++ {
		child := items.Child(i)
		if m.Section != "" && c.Type(child) == m.Section {
			private = m.SectionPrivate(c, child)
			continue
		}
		if private || (m.Private != nil && m.Private(c, n, child)) {
			continue
		}
		from := startRow(child)
		if len(m.Annotations) > 0 {
			from = precedingSiblingsRow(items, child, i, m.Annotations, c.Lang)
		}
		switch m.kind(c, child) {
		case Field:
			if c.Structs {
				inner = c.EmitWithDoc(inner, from, endRow(child))
			}
		case Method:
			if c.Functions {
				inner = c.EmitLeadingDoc(inner, from)
				inner = EmitDecorators(inner, c.Lines, from, startRow(child))
				inner = EmitFuncSig(inner, c.Lines, child, c.Lang, false, m.MethodBody)
			}
		case Nested:
			if sub := m.Emit(c, nil, child); len(sub) > 0 {
				inner = c.EmitLeadingDoc(inner, from)
				inner = EmitDecorators(inner, c.Lines, from, startRow(child))
				inner = append(inner, sub...)
			}
		}
	}
	if !c.Structs && len(inner) == 0 {
		return out
	}

	out = internal.KeepLines(out, c.Lines, startRow(n), headerEnd)
	out = append(out, inner...)
	if closingRow := endRow(body); closingRow > headerEnd {
		out = internal.KeepLine(out, c.Lines, closingRow)
	}
	return out
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
