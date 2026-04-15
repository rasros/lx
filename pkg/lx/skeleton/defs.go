package skeleton

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/rasros/lx/pkg/lx/skeleton/langs"
)

type langDef struct {
	newLang     func() *gotreesitter.Language
	funcTypes   []string
	structTypes []string

	indentBody    bool
	bodyChildType string
	singleLineSig bool

	decoratedType   string
	definitionField string

	iterateChildren []string

	funcVisible func(node *gotreesitter.Node, src []byte, lang *gotreesitter.Language) bool

	emitStruct func(out, src []byte, lines [][]byte, node *gotreesitter.Node, lang *gotreesitter.Language, functions bool) []byte
}

var langDefs = map[string]langDef{
	"go": {
		newLang:     grammars.GoLanguage,
		funcTypes:   []string{"function_declaration", "method_declaration"},
		structTypes: []string{"type_declaration"},
		funcVisible: langs.GoFuncVisible,
		emitStruct:  langs.GoEmitTypeDecl,
	},

	"python": {
		newLang:         grammars.PythonLanguage,
		funcTypes:       []string{"function_definition"},
		structTypes:     []string{"class_definition"},
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
		funcVisible: langs.CFuncVisible,
		emitStruct:  langs.CEmitTypedef,
	},

	"cpp": {
		newLang:     grammars.CppLanguage,
		funcTypes:   []string{"function_definition", "declaration"},
		structTypes: []string{"type_definition", "struct_specifier", "class_specifier", "union_specifier", "enum_specifier"},
		funcVisible: langs.CppFuncVisible,
		emitStruct:  langs.CppEmitClassOrStruct,
	},

	"java": {
		newLang:     grammars.JavaLanguage,
		structTypes: []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"},
		emitStruct:  langs.JavaEmitClass,
	},

	"rust": {
		newLang:     grammars.RustLanguage,
		funcTypes:   []string{"function_item"},
		structTypes: []string{"struct_item", "enum_item", "trait_item", "impl_item"},
		funcVisible: langs.RustFuncVisible,
		emitStruct:  langs.RustEmitItem,
	},

	"typescript": {
		newLang:         grammars.TypescriptLanguage,
		funcTypes:       []string{"function_declaration"},
		structTypes:     []string{"class_declaration", "interface_declaration"},
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitStruct:      langs.TSEmitClassOrInterface,
	},

	"tsx": {
		newLang:         grammars.TsxLanguage,
		funcTypes:       []string{"function_declaration"},
		structTypes:     []string{"class_declaration", "interface_declaration"},
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitStruct:      langs.TSEmitClassOrInterface,
	},

	"javascript": {
		newLang:         grammars.JavascriptLanguage,
		funcTypes:       []string{"function_declaration"},
		structTypes:     []string{"class_declaration"},
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitStruct:      langs.TSEmitClassOrInterface,
	},

	"jsx": {
		newLang:         grammars.JavascriptLanguage,
		funcTypes:       []string{"function_declaration"},
		structTypes:     []string{"class_declaration"},
		decoratedType:   "export_statement",
		definitionField: "declaration",
		emitStruct:      langs.TSEmitClassOrInterface,
	},

	"kotlin": {
		newLang:       grammars.KotlinLanguage,
		funcTypes:     []string{"function_declaration"},
		structTypes:   []string{"class_declaration", "infix_expression"},
		bodyChildType: "function_body",
		funcVisible:   langs.KtFuncVisible,
		emitStruct:    langs.KotlinEmitClass,
	},

	"csharp": {
		newLang:     grammars.CSharpLanguage,
		structTypes: []string{"class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "record_declaration"},
		emitStruct:  langs.CSharpEmitClass,
	},

	"ruby": {
		newLang:     grammars.RubyLanguage,
		funcTypes:   []string{"method"},
		structTypes: []string{"class", "module"},
		indentBody:  true,
		emitStruct:  langs.RubyEmitClassOrModule,
	},

	"swift": {
		newLang:     grammars.SwiftLanguage,
		funcTypes:   []string{"function_declaration", "init_declaration"},
		structTypes: []string{"class_declaration", "protocol_declaration", "extension_declaration"},
		funcVisible: langs.SwiftFuncVisible,
		emitStruct:  langs.SwiftEmitType,
	},

	"scala": {
		newLang:     grammars.ScalaLanguage,
		funcTypes:   []string{"function_definition"},
		structTypes: []string{"class_definition", "object_definition", "trait_definition"},
		funcVisible: langs.ScalaFuncVisible,
		emitStruct:  langs.ScalaEmitType,
	},

	"php": {
		newLang:     grammars.PhpLanguage,
		funcTypes:   []string{"function_definition"},
		structTypes: []string{"class_declaration", "interface_declaration"},
		emitStruct:  langs.PHPEmitClass,
	},

	"bash": {
		newLang:       grammars.BashLanguage,
		funcTypes:     []string{"function_definition"},
		bodyChildType: "compound_statement",
	},

	"powershell": {
		newLang:         grammars.PowershellLanguage,
		funcTypes:       []string{"function_statement"},
		bodyChildType:   "{",
		iterateChildren: []string{"statement_list"},
	},

	"dart": {
		newLang:     grammars.DartLanguage,
		funcTypes:   []string{"function_signature"},
		structTypes: []string{"class_definition"},
		emitStruct:  langs.DartEmitClass,
	},

	"lua": {
		newLang:       grammars.LuaLanguage,
		funcTypes:     []string{"function_declaration"},
		indentBody:    true,
		bodyChildType: "end",
	},

	"julia": {
		newLang:       grammars.JuliaLanguage,
		funcTypes:     []string{"function_definition"},
		structTypes:   []string{"struct_definition", "abstract_definition"},
		indentBody:    true,
		bodyChildType: "block",
		emitStruct:    langs.JuliaEmitStruct,
	},

	"zig": {
		newLang:       grammars.ZigLanguage,
		funcTypes:     []string{"Decl"},
		bodyChildType: "Block",
		funcVisible:   langs.ZigFuncVisible,
	},

	"haskell": {
		newLang:         grammars.HaskellLanguage,
		funcTypes:       []string{"function"},
		structTypes:     []string{"data_type", "newtype"},
		indentBody:      true,
		bodyChildType:   "match",
		iterateChildren: []string{"declarations"},
		emitStruct:      langs.HaskellEmitType,
	},

	"groovy": {
		newLang:       grammars.GroovyLanguage,
		funcTypes:     []string{"function_definition"},
		structTypes:   []string{"class_definition"},
		bodyChildType: "closure",
		emitStruct:    langs.GroovyEmitClass,
	},

	"perl": {
		newLang:       grammars.PerlLanguage,
		funcTypes:     []string{"subroutine_declaration_statement"},
		bodyChildType: "block",
	},

	"objectivec": {
		newLang:       grammars.ObjcLanguage,
		funcTypes:     []string{"function_definition"},
		structTypes:   []string{"class_interface", "protocol_declaration"},
		bodyChildType: "compound_statement",
		emitStruct:    langs.ObjCEmitInterface,
	},

	"ocaml": {
		newLang:     grammars.OcamlLanguage,
		structTypes: []string{"value_definition", "type_definition", "module_definition", "module_type_definition"},
		emitStruct:  langs.OCamlEmitDefinition,
	},

	"pascal": {
		newLang:         grammars.PascalLanguage,
		structTypes:     []string{"defProc", "declTypes"},
		iterateChildren: []string{"program"},
		emitStruct:      langs.PascalEmitDecl,
	},

	"lisp": {
		newLang:       grammars.CommonlispLanguage,
		funcTypes:     []string{"list_lit"},
		singleLineSig: true,
		funcVisible:   langs.CLFuncVisible,
	},

	"ada": {
		newLang:         grammars.AdaLanguage,
		funcTypes:       []string{"subprogram_body"},
		structTypes:     []string{"package_declaration", "subprogram_declaration"},
		iterateChildren: []string{"compilation", "compilation_unit"},
		emitStruct:      langs.AdaEmitDecl,
		funcVisible:     langs.AdaFuncVisible,
	},
}
