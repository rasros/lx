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
// A type declaration is emitted by emitStruct if set, else by the generic
// member walker configured in members, else kept whole with -Y.
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

	funcVisible func(node *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool

	members    *langs.Members
	emitFunc   langs.Emitter
	emitStruct langs.Emitter
}

var (
	javaMembers = &langs.Members{
		Bodies:  []string{"class_body", "interface_body", "enum_body"},
		Fields:  []string{"field_declaration"},
		Methods: []string{"method_declaration", "constructor_declaration"},
		Nested:  []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"},
		Private: langs.JavaPrivate,
	}

	csharpMembers = &langs.Members{
		BodyField: "body",
		Bodies:    []string{"declaration_list"},
		Fields:    []string{"field_declaration", "property_declaration"},
		Methods:   []string{"method_declaration", "constructor_declaration"},
		Nested:    []string{"class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "record_declaration"},
		Private:   langs.CSharpPrivate,
	}

	scalaMembers = &langs.Members{
		Bodies:  []string{"template_body"},
		Fields:  []string{"val_definition", "var_definition"},
		Methods: []string{"function_definition", "function_declaration"},
		Nested:  []string{"class_definition", "object_definition", "trait_definition"},
		Private: langs.ScalaPrivate,
	}

	kotlinMembers = &langs.Members{
		Bodies:     []string{"class_body"},
		Fields:     []string{"property_declaration"},
		Methods:    []string{"function_declaration"},
		Nested:     []string{"class_declaration", "object_declaration"},
		MethodBody: "function_body",
		Private:    langs.KotlinPrivate,
	}

	swiftMembers = &langs.Members{
		Bodies:  []string{"class_body", "protocol_body"},
		Fields:  []string{"property_declaration", "protocol_property_declaration"},
		Methods: []string{"function_declaration", "init_declaration", "protocol_function_declaration"},
		Nested:  []string{"class_declaration"},
		Private: langs.SwiftPrivate,
	}

	phpMembers = &langs.Members{
		Bodies:  []string{"declaration_list"},
		Fields:  []string{"property_declaration", "const_declaration"},
		Methods: []string{"method_declaration"},
		Private: langs.PHPPrivate,
	}

	dartMembers = &langs.Members{
		BodyField:   "body",
		Kind:        langs.DartMember,
		Annotations: []string{"annotation", "decorator"},
		Private:     langs.DartPrivate,
	}

	groovyMembers = &langs.Members{
		Bodies:     []string{"closure"},
		Fields:     []string{"declaration"},
		Methods:    []string{"function_definition"},
		MethodBody: "closure",
		Private:    langs.GroovyPrivate,
	}

	// tsMembers covers classes and interfaces of TypeScript and JavaScript.
	tsMembers = &langs.Members{
		BodyField:   "body",
		Bodies:      []string{"class_body", "interface_body"},
		Fields:      []string{"public_field_definition", "field_definition", "property_signature", "index_signature"},
		Methods:     []string{"method_definition", "method_signature", "call_signature", "construct_signature"},
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
		Bodies:           []string{"field_declaration_list"},
		SkipBodiless:     true,
		Kind:             langs.CppMember,
		Section:          "access_specifier",
		SectionPrivate:   langs.CppSectionPrivate,
		PrivateByDefault: []string{"class_specifier"},
	}
)

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

	"c": {
		newLang:     grammars.CLanguage,
		funcTypes:   []string{"function_definition", "declaration"},
		structTypes: []string{"type_definition"},
		comments:    &langs.CComments,
		funcVisible: langs.CFuncVisible,
		emitStruct:  langs.CEmitTypedef,
	},

	"cpp": {
		newLang:     grammars.CppLanguage,
		funcTypes:   []string{"function_definition", "declaration"},
		structTypes: []string{"type_definition", "struct_specifier", "class_specifier", "union_specifier", "enum_specifier"},
		comments:    &langs.CComments,
		funcVisible: langs.CppFuncVisible,
		emitStruct:  langs.CppEmitter(cppMembers),
	},

	"java": {
		newLang:     grammars.JavaLanguage,
		structTypes: []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"},
		comments:    &langs.CComments,
		members:     javaMembers,
	},

	"rust": {
		newLang:     grammars.RustLanguage,
		funcTypes:   []string{"function_item"},
		structTypes: []string{"struct_item", "enum_item", "trait_item", "impl_item"},
		comments:    &langs.RustComments,
		funcVisible: langs.RustFuncVisible,
		members:     rustMembers,
	},

	"typescript": {
		newLang:         grammars.TypescriptLanguage,
		funcTypes:       []string{"function_declaration", "lexical_declaration"},
		structTypes:     []string{"class_declaration", "interface_declaration"},
		comments:        &langs.CComments,
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitFunc:        langs.TSEmitFunc,
		members:         tsMembers,
	},

	"tsx": {
		newLang:         grammars.TsxLanguage,
		funcTypes:       []string{"function_declaration", "lexical_declaration"},
		structTypes:     []string{"class_declaration", "interface_declaration"},
		comments:        &langs.CComments,
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitFunc:        langs.TSEmitFunc,
		members:         tsMembers,
	},

	"javascript": {
		newLang:         grammars.JavascriptLanguage,
		funcTypes:       []string{"function_declaration", "lexical_declaration"},
		structTypes:     []string{"class_declaration"},
		comments:        &langs.CComments,
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitFunc:        langs.TSEmitFunc,
		members:         tsMembers,
	},

	"jsx": {
		newLang:         grammars.JavascriptLanguage,
		funcTypes:       []string{"function_declaration", "lexical_declaration"},
		structTypes:     []string{"class_declaration"},
		comments:        &langs.CComments,
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitFunc:        langs.TSEmitFunc,
		members:         tsMembers,
	},

	"kotlin": {
		newLang:       grammars.KotlinLanguage,
		funcTypes:     []string{"function_declaration"},
		structTypes:   []string{"class_declaration", "object_declaration", "infix_expression"},
		comments:      &langs.CComments,
		bodyChildType: "function_body",
		funcVisible:   langs.KtFuncVisible,
		emitStruct:    langs.KotlinEmitter(kotlinMembers),
	},

	"csharp": {
		newLang:     grammars.CSharpLanguage,
		structTypes: []string{"class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "record_declaration"},
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
		structTypes: []string{"class_declaration", "protocol_declaration", "extension_declaration"},
		comments:    &langs.CComments,
		funcVisible: langs.SwiftFuncVisible,
		members:     swiftMembers,
	},

	"scala": {
		newLang:     grammars.ScalaLanguage,
		funcTypes:   []string{"function_definition"},
		structTypes: []string{"class_definition", "object_definition", "trait_definition"},
		comments:    &langs.CComments,
		funcVisible: langs.ScalaFuncVisible,
		members:     scalaMembers,
	},

	"php": {
		newLang:     grammars.PhpLanguage,
		funcTypes:   []string{"function_definition"},
		structTypes: []string{"class_declaration", "interface_declaration"},
		comments:    &langs.PHPComments,
		members:     phpMembers,
	},

	"dart": {
		newLang:     grammars.DartLanguage,
		funcTypes:   []string{"function_signature"},
		structTypes: []string{"class_definition"},
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
		structTypes:     []string{"data_type", "newtype"},
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
	},

	"ocaml": {
		newLang:     grammars.OcamlLanguage,
		structTypes: []string{"value_definition", "type_definition", "module_definition", "module_type_definition"},
		comments:    &langs.OCamlComments,
		emitStruct:  langs.OCamlEmitDefinition,
	},
}
