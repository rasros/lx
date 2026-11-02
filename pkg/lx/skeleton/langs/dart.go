package langs

import (
	"slices"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// DartPrivate reports a library-private member: its name starts with "_".
func DartPrivate(c *Ctx, _, n *gotreesitter.Node) bool {
	row := startRow(n)
	if row >= len(c.Lines) {
		return false
	}
	line := strings.TrimSpace(string(c.Lines[row]))
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if idx := strings.IndexAny(line, "(;{="); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	return isPrivateName(parts[len(parts)-1])
}

// dartSignatures are the children of a Dart class-body declaration that make
// it a method or constructor declared without a body rather than a field.
var dartSignatures = []string{
	"function_signature", "getter_signature", "setter_signature", "operator_signature",
	"constructor_signature", "constant_constructor_signature",
	"factory_constructor_signature", "redirecting_factory_constructor_signature",
}

// DartMember classifies a Dart class-body member. A method with a body is a
// method_signature (its function_body is the next sibling); one without is a
// declaration holding a signature. Any other declaration is a field.
func DartMember(c *Ctx, n *gotreesitter.Node) MemberKind {
	switch c.Type(n) {
	case "method_signature":
		return Method
	case "declaration":
		for i := 0; i < n.ChildCount(); i++ {
			if slices.Contains(dartSignatures, c.Type(n.Child(i))) {
				return Method
			}
		}
		return Field
	}
	return Skip
}
