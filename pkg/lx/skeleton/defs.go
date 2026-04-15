package skeleton

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/rasros/lx/pkg/lx/skeleton/langs"
)

type langDef struct {
	newLang func() *gotreesitter.Language
	funcTypes []string
	structTypes []string

	indentBody bool

	decoratedType   string
	definitionField string

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
		funcTypes:   nil,
		structTypes: []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"},
		funcVisible: nil,
		emitStruct:  langs.JavaEmitClass,
	},
}
