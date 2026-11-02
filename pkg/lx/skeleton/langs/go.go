package langs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
)

func GoFuncVisible(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool {
	name := n.ChildByFieldName("name", lang)
	return name != nil && isUppercase(name.Text(src))
}

// GoEmitTypeDecl emits a Go type declaration. It stays custom because one
// declaration can group several type specs in `type ( ... )`, each with its
// own doc, and the group's parentheses are kept only around specs that keep
// something. The body of a spec is not a child of the spec either but of its
// struct_type or interface_type, and visibility is the case of the name.
func GoEmitTypeDecl(c *Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	grouped := goIsGroupedTypeDecl(c, n)
	var groupMembers []internal.LineRange

	for i := 0; i < n.ChildCount(); i++ {
		spec := n.Child(i)
		if c.Type(spec) != "type_spec" {
			continue
		}

		if grouped {
			before := len(groupMembers)
			groupMembers = goEmitTypeSpec(c, groupMembers, spec)
			if len(groupMembers) > before {
				groupMembers = c.EmitLeadingDoc(groupMembers, startRow(spec))
			}
		} else {
			out = goEmitTypeSpec(c, out, spec)
		}
	}

	if grouped && len(groupMembers) > 0 {
		out = internal.KeepLine(out, c.Lines, startRow(n))
		out = append(out, groupMembers...)
		out = internal.KeepLine(out, c.Lines, endRow(n))
	}
	return out
}

func goEmitTypeSpec(c *Ctx, out []internal.LineRange, spec *gotreesitter.Node) []internal.LineRange {
	nameNode := spec.ChildByFieldName("name", c.Lang)
	if nameNode == nil || !isUppercase(c.Text(nameNode)) {
		return out
	}

	typeNode := spec.ChildByFieldName("type", c.Lang)
	if typeNode == nil {
		return out
	}

	switch c.Type(typeNode) {
	case "interface_type":
		return goEmitInterface(c, out, spec, typeNode)
	case "struct_type":
		if c.Structs {
			return goEmitStruct(c, out, spec, typeNode)
		}
	default:
		if c.Structs {
			return c.KeepNode(out, spec)
		}
	}
	return out
}

func goEmitStruct(c *Ctx, out []internal.LineRange, specNode, structNode *gotreesitter.Node) []internal.LineRange {
	fieldList := findChildByType(structNode, "field_declaration_list", c.Lang)
	if fieldList == nil {
		return c.KeepNode(out, specNode)
	}
	headerStart := startRow(specNode)
	out = internal.KeepLine(out, c.Lines, headerStart)

	for i := 0; i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(i)
		if c.Type(child) == "field_declaration" && goFieldIsExported(c, child) && startRow(child) != headerStart {
			out = c.EmitWithDoc(out, startRow(child), endRow(child))
		}
	}

	if closingRow := endRow(fieldList); closingRow > headerStart {
		out = internal.KeepLine(out, c.Lines, closingRow)
	}
	return out
}

// goEmitInterface keeps an interface's exported methods with functions; its
// header and closing row with types, or with functions when it keeps a
// method.
func goEmitInterface(c *Ctx, out []internal.LineRange, specNode, ifaceNode *gotreesitter.Node) []internal.LineRange {
	headerStart := startRow(specNode)

	var methods []internal.LineRange
	if c.Functions {
		for i := 0; i < ifaceNode.ChildCount(); i++ {
			child := ifaceNode.Child(i)
			t := c.Type(child)
			if t != "method_elem" && t != "method_spec" {
				continue
			}
			nameNode := findChildByType(child, "field_identifier", c.Lang)
			if nameNode != nil && isUppercase(c.Text(nameNode)) && startRow(child) != headerStart {
				methods = c.EmitWithDoc(methods, startRow(child), endRow(child))
			}
		}
	}
	if !c.Structs && len(methods) == 0 {
		return out
	}

	out = internal.KeepLine(out, c.Lines, headerStart)
	out = append(out, methods...)
	if closingRow := endRow(ifaceNode); closingRow > headerStart {
		out = internal.KeepLine(out, c.Lines, closingRow)
	}
	return out
}

func goIsGroupedTypeDecl(c *Ctx, n *gotreesitter.Node) bool {
	for i := 0; i < n.ChildCount(); i++ {
		if c.Type(n.Child(i)) == "(" {
			return true
		}
	}
	return false
}

func goFieldIsExported(c *Ctx, field *gotreesitter.Node) bool {
	if name := field.ChildByFieldName("name", c.Lang); name != nil {
		return isUppercase(c.Text(name))
	}
	for i := 0; i < field.ChildCount(); i++ {
		child := field.Child(i)
		t := c.Type(child)
		if t == "field_identifier" || t == "type_identifier" {
			text := strings.TrimLeft(c.Text(child), "*")
			if dot := strings.LastIndex(text, "."); dot >= 0 {
				text = text[dot+1:]
			}
			return isUppercase(text)
		}
	}
	return false
}
