package skeleton

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/rasros/lx/pkg/lx/skeleton/langs"
)

// langDef describes one language's skeleton. Top-level nodes of funcTypes
// keep their signature with -u (--functions); nodes of structTypes are type
// declarations, walked with both flags:
//
//   - -u alone keeps function and method signatures, and the header and
//     closing rows of the types that hold kept methods;
//   - -Y alone keeps types and their fields;
//   - both keep the full skeleton.
//
// A type declaration is emitted by emitStruct, else by members, else kept
// whole with -Y.
type langDef struct {
	newLang     func() *gotreesitter.Language
	funcTypes   []string
	structTypes []string
	// comments is what counts as a declaration's leading doc block.
	comments *langs.CommentSyntax

	indentBody    bool
	bodyChildType string

	decoratedType   string
	definitionField string

	iterateChildren []string
	// namespaces are node types whose "body" holds declarations walked like
	// top-level ones, kept between the namespace's header and closing row.
	namespaces []string
	// conditionals keeps the directive rows of the preprocessor
	// conditionals around kept rows (see langs.BalanceConditionals).
	conditionals bool
	// recoverErrors walks the declarations inside error and hidden nodes.
	recoverErrors bool
	// unwrap returns the declaration a top-level node wraps (a C++
	// template_declaration's class or function), or the node itself. Rows
	// of the wrapper before the declaration are kept like decorators.
	unwrap func(c *langs.Ctx, n *gotreesitter.Node) *gotreesitter.Node

	funcVisible func(c *langs.Ctx, node *gotreesitter.Node) bool

	members    *langs.Members
	emitFunc   langs.Emitter
	emitStruct langs.Emitter
}

var (
	javaTypes = []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration"}

	javaMembers = &langs.Members{
		Bodies:  []string{"class_body", "interface_body", "enum_body", "annotation_type_body"},
		Groups:  []string{"enum_body_declarations"},
		Fields:  []string{"field_declaration", "constant_declaration", "enum_constant"},
		Methods: []string{"method_declaration", "constructor_declaration", "compact_constructor_declaration", "annotation_type_element_declaration"},
		Nested:  javaTypes,
		Private: langs.JavaPrivate,
	}

	csharpTypes = []string{"class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "record_declaration"}

	// preprocGroups are conditional compilation blocks, whose members
	// belong to the body around them; their #if, #else and #endif lines are
	// kept around the members kept inside.
	preprocGroups = []string{"preproc_if", "preproc_ifdef", "preproc_elif", "preproc_elifdef", "preproc_else"}

	csharpMembers = &langs.Members{
		BodyField: "body",
		Bodies:    []string{"declaration_list"},
		Fields: []string{"field_declaration", "property_declaration", "event_field_declaration", "event_declaration",
			"delegate_declaration", "enum_member_declaration"},
		Methods: []string{"method_declaration", "constructor_declaration", "destructor_declaration", "operator_declaration",
			"conversion_operator_declaration", "indexer_declaration"},
		Nested:       csharpTypes,
		Groups:       preprocGroups,
		Frames:       preprocGroups,
		MethodBodies: []string{"accessor_list"},
		Private:      langs.CSharpPrivate,
	}

	scalaMembers = &langs.Members{
		Bodies:  []string{"template_body"},
		Fields:  []string{"val_definition", "var_definition"},
		Methods: []string{"function_definition", "function_declaration"},
		Nested:  []string{"class_definition", "object_definition", "trait_definition"},
		Private: langs.ScalaPrivate,
	}

	kotlinMembers = &langs.Members{
		Bodies:  []string{"class_body", "enum_class_body"},
		Fields:  []string{"property_declaration", "enum_entry"},
		Methods: []string{"function_declaration", "secondary_constructor"},
		Nested:  []string{"class_declaration", "object_declaration", "companion_object"},
		// A secondary constructor's body is bare braces around statements.
		MethodBodies: []string{"function_body", "{"},
		Private:      langs.KotlinPrivate,
	}

	swiftMembers = &langs.Members{
		Bodies: []string{"class_body", "protocol_body", "enum_class_body"},
		Fields: []string{"property_declaration", "protocol_property_declaration", "enum_entry"},
		Methods: []string{"function_declaration", "init_declaration", "deinit_declaration", "subscript_declaration",
			"protocol_function_declaration"},
		Nested:       []string{"class_declaration"},
		MethodBodies: []string{"computed_property"},
		Private:      langs.SwiftPrivate,
	}

	phpMembers = &langs.Members{
		Bodies:  []string{"declaration_list", "enum_declaration_list"},
		Fields:  []string{"property_declaration", "const_declaration", "enum_case"},
		Methods: []string{"method_declaration"},
		Private: langs.PHPPrivate,
	}

	dartMembers = &langs.Members{
		BodyField:   "body",
		Bodies:      []string{"class_body", "extension_body", "enum_body"},
		Kind:        langs.DartMember,
		Annotations: []string{"annotation", "decorator"},
		Private:     langs.DartPrivate,
	}

	groovyMembers = &langs.Members{
		Bodies:       []string{"closure"},
		Fields:       []string{"declaration"},
		Methods:      []string{"function_definition"},
		MethodBodies: []string{"closure"},
		Private:      langs.GroovyPrivate,
	}

	tsTypes = []string{"class_declaration", "abstract_class_declaration", "interface_declaration", "enum_declaration",
		"type_alias_declaration"}

	// tsMembers covers classes, interfaces and enums of TypeScript and
	// JavaScript.
	tsMembers = &langs.Members{
		BodyField: "body",
		Bodies:    []string{"class_body", "interface_body", "enum_body"},
		Fields: []string{"public_field_definition", "field_definition", "property_signature", "index_signature",
			"property_identifier", "enum_assignment"},
		Methods:     []string{"method_definition", "method_signature", "abstract_method_signature", "call_signature", "construct_signature"},
		Annotations: []string{"decorator"},
		Private:     langs.TSPrivate,
	}

	// rustMembers covers struct fields, enum variants, and trait and impl
	// functions.
	rustMembers = &langs.Members{
		Bodies:  []string{"field_declaration_list", "enum_variant_list", "declaration_list"},
		Fields:  []string{"field_declaration", "enum_variant"},
		Methods: []string{"function_item", "function_signature_item"},
		Private: langs.RustPrivate,
	}

	cppMembers = &langs.Members{
		Bodies:           []string{"field_declaration_list", "enumerator_list"},
		SkipBodiless:     true,
		Unwrap:           langs.CppUnwrap,
		Kind:             langs.CppMember,
		Groups:           preprocGroups,
		Frames:           preprocGroups,
		Section:          "access_specifier",
		SectionPrivate:   langs.CppSectionPrivate,
		PrivateByDefault: []string{"class_specifier"},
	}

	objcGroups = append([]string{"instance_variables", "qualified_protocol_interface_declaration"}, preprocGroups...)

	// objcMembers walks an @interface or @protocol, whose members are its
	// own children up to @end. Instance variables are hidden after @private
	// or @package.
	objcMembers = &langs.Members{
		BodyIsNode:     true,
		Groups:         objcGroups,
		Frames:         objcGroups,
		SectionScopes:  []string{"instance_variables"},
		Unwrap:         langs.ObjCUnwrap,
		Fields:         []string{"property_declaration", "struct_declaration", "declaration"},
		Methods:        []string{"method_declaration"},
		Section:        "visibility_specification",
		SectionPrivate: langs.ObjCSectionPrivate,
	}
)

// tsDef is the langDef of TypeScript, JavaScript or one of their JSX
// dialects, whose type declarations are structTypes.
func tsDef(newLang func() *gotreesitter.Language, structTypes []string) langDef {
	return langDef{
		newLang:         newLang,
		funcTypes:       []string{"function_declaration", "lexical_declaration"},
		structTypes:     structTypes,
		comments:        &langs.CComments,
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitFunc:        langs.TSEmitFunc,
		members:         tsMembers,
	}
}

var langDefs = map[string]langDef{
	"go": {
		newLang:     grammars.GoLanguage,
		funcTypes:   []string{"function_declaration", "method_declaration"},
		structTypes: []string{"type_declaration"},
		comments:    &langs.CComments,
		funcVisible: langs.GoFuncVisible,
		emitStruct:  langs.GoEmitTypeDecl,
	},

	"python": {
		newLang:         grammars.PythonLanguage,
		funcTypes:       []string{"function_definition"},
		structTypes:     []string{"class_definition"},
		comments:        &langs.HashComments,
		indentBody:      true,
		decoratedType:   "decorated_definition",
		definitionField: "definition",
		funcVisible:     langs.PyFuncVisible,
		emitStruct:      langs.PyEmitClass,
	},

	// C's structs, unions and enums are walked by the C++ member table,
	// whose C++-only parts never match C source.
	"c": {
		newLang:     grammars.CLanguage,
		funcTypes:   []string{"function_definition", "declaration"},
		structTypes: []string{"type_definition", "declaration", "struct_specifier", "union_specifier", "enum_specifier"},
		comments:    &langs.CComments,
		funcVisible: langs.CFuncVisible,
		namespaces:  []string{"linkage_specification"},
		unwrap:      langs.CppUnwrap,
		emitStruct:  langs.CppEmitter(cppMembers),

		iterateChildren: preprocGroups,
		conditionals:    true,
		recoverErrors:   true,
	},

	"cpp": {
		newLang:     grammars.CppLanguage,
		funcTypes:   []string{"function_definition", "declaration"},
		structTypes: []string{"type_definition", "alias_declaration", "declaration", "struct_specifier", "class_specifier", "union_specifier", "enum_specifier"},
		comments:    &langs.CComments,
		funcVisible: langs.CFuncVisible,
		namespaces:  []string{"namespace_definition", "linkage_specification"},
		unwrap:      langs.CppUnwrap,
		emitStruct:  langs.CppEmitter(cppMembers),

		iterateChildren: preprocGroups,
		conditionals:    true,
		recoverErrors:   true,
	},

	"java": {
		newLang:     grammars.JavaLanguage,
		structTypes: javaTypes,
		comments:    &langs.CComments,
		members:     javaMembers,
	},

	"rust": {
		newLang:     grammars.RustLanguage,
		funcTypes:   []string{"function_item"},
		structTypes: []string{"struct_item", "enum_item", "trait_item", "impl_item", "type_item"},
		comments:    &langs.RustComments,
		funcVisible: langs.RustFuncVisible,
		members:     rustMembers,
	},

	"typescript": tsDef(grammars.TypescriptLanguage, tsTypes),
	"tsx":        tsDef(grammars.TsxLanguage, tsTypes),
	"javascript": tsDef(grammars.JavascriptLanguage, []string{"class_declaration"}),
	"jsx":        tsDef(grammars.JavascriptLanguage, []string{"class_declaration"}),

	"kotlin": {
		newLang:       grammars.KotlinLanguage,
		funcTypes:     []string{"function_declaration"},
		structTypes:   []string{"class_declaration", "object_declaration", "infix_expression", "type_alias"},
		comments:      &langs.CComments,
		bodyChildType: "function_body",
		funcVisible:   langs.KtFuncVisible,
		emitStruct:    langs.KotlinEmitter(kotlinMembers),
	},

	"csharp": {
		newLang:     grammars.CSharpLanguage,
		structTypes: csharpTypes,
		namespaces:  []string{"namespace_declaration"},
		comments:    &langs.CComments,
		members:     csharpMembers,
	},

	"ruby": {
		newLang:     grammars.RubyLanguage,
		funcTypes:   []string{"method"},
		structTypes: []string{"class", "module"},
		comments:    &langs.RubyComments,
		indentBody:  true,
		emitStruct:  langs.RubyEmitClassOrModule,
	},

	"swift": {
		newLang:     grammars.SwiftLanguage,
		funcTypes:   []string{"function_declaration", "init_declaration"},
		structTypes: []string{"class_declaration", "protocol_declaration", "extension_declaration", "typealias_declaration"},
		comments:    &langs.CComments,
		funcVisible: langs.SwiftFuncVisible,
		members:     swiftMembers,
	},

	"scala": {
		newLang:     grammars.ScalaLanguage,
		funcTypes:   []string{"function_definition"},
		structTypes: []string{"class_definition", "object_definition", "trait_definition", "type_definition"},
		comments:    &langs.CComments,
		funcVisible: langs.ScalaFuncVisible,
		members:     scalaMembers,
	},

	"php": {
		newLang:     grammars.PhpLanguage,
		funcTypes:   []string{"function_definition"},
		structTypes: []string{"class_declaration", "interface_declaration", "trait_declaration", "enum_declaration"},
		comments:    &langs.PHPComments,
		members:     phpMembers,
	},

	"dart": {
		newLang:     grammars.DartLanguage,
		funcTypes:   []string{"function_signature"},
		structTypes: []string{"class_definition", "mixin_declaration", "extension_declaration", "enum_declaration", "type_alias"},
		comments:    &langs.CComments,
		members:     dartMembers,
	},

	"zig": {
		newLang:       grammars.ZigLanguage,
		funcTypes:     []string{"function_declaration"},
		comments:      &langs.ZigComments,
		bodyChildType: "block",
		funcVisible:   langs.ZigFuncVisible,
	},

	"haskell": {
		newLang:         grammars.HaskellLanguage,
		funcTypes:       []string{"signature", "function"},
		structTypes:     []string{"data_type", "newtype", "type_synomym"},
		comments:        &langs.HaskellComments,
		indentBody:      true,
		bodyChildType:   "match",
		iterateChildren: []string{"declarations"},
	},

	"groovy": {
		newLang:       grammars.GroovyLanguage,
		funcTypes:     []string{"function_definition"},
		structTypes:   []string{"class_definition"},
		comments:      &langs.CComments,
		bodyChildType: "closure",
		members:       groovyMembers,
	},

	"objectivec": {
		newLang:       grammars.ObjcLanguage,
		funcTypes:     []string{"function_definition"},
		structTypes:   []string{"class_interface", "protocol_declaration"},
		comments:      &langs.CComments,
		bodyChildType: "compound_statement",
		members:       objcMembers,
	},

	"ocaml": {
		newLang:     grammars.OcamlLanguage,
		structTypes: []string{"value_definition", "type_definition", "module_definition", "module_type_definition"},
		comments:    &langs.OCamlComments,
		emitStruct:  langs.OCamlEmitDefinition,
	},
}
