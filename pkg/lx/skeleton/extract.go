package skeleton

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal/content"
)

func extract(langName string, src []byte, functions, structs bool) []byte {
	def, ok := langDefs[langName]
	if !ok {
		return src
	}
	lang := def.newLang()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return src
	}
	root := tree.RootNode()
	lines := content.SplitLines(src)
	var out []byte

	for i := 0; i < root.ChildCount(); i++ {
		node := root.Child(i)

		if def.decoratedType != "" && node.Type(lang) == def.decoratedType {
			if defNode := node.ChildByFieldName(def.definitionField, lang); defNode != nil {
				node = defNode
			}
		}

		nodeType := node.Type(lang)

		switch {
		case functions && containsStr(def.funcTypes, nodeType):
			if def.funcVisible == nil || def.funcVisible(node, src, lang) {
				out = emitFuncSig(out, lines, node, lang, def.indentBody, def.bodyChildType)
			}
		case structs && containsStr(def.structTypes, nodeType):
			out = def.emitStruct(out, src, lines, node, lang, functions)
		}
	}
	return out
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
