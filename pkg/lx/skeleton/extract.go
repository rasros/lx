package skeleton

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/rasros/lx/pkg/lx/skeleton/langs"
	"sync"
)

type parserRuntime struct {
	lang *gotreesitter.Language
	pool *gotreesitter.ParserPool
}

var (
	parserRuntimes sync.Map // map[string]*parserRuntime
	parserInitMu   sync.Mutex
)

func extract(langName string, src []byte, functions, structs bool) (out []byte) {
	defer func() {
		if r := recover(); r != nil {
			out = src
		}
	}()

	def, ok := langDefs[langName]
	if !ok {
		return src
	}
	pool, lang := parserPoolFor(langName, def.newLang)

	tree, err := pool.Parse(src)
	if err != nil {
		return src
	}
	defer tree.Release()

	root := tree.RootNode()
	lines := internal.SplitLines(src)

	for i := 0; i < root.ChildCount(); i++ {
		out = def.processNode(root.Child(i), out, src, lines, lang, functions, structs)
	}
	return out
}

func parserPoolFor(langName string, newLang func() *gotreesitter.Language) (*gotreesitter.ParserPool, *gotreesitter.Language) {
	if v, ok := parserRuntimes.Load(langName); ok {
		rt := v.(*parserRuntime)
		return rt.pool, rt.lang
	}

	parserInitMu.Lock()
	defer parserInitMu.Unlock()

	if v, ok := parserRuntimes.Load(langName); ok {
		rt := v.(*parserRuntime)
		return rt.pool, rt.lang
	}

	lang := newLang()
	rt := &parserRuntime{
		lang: lang,
		pool: gotreesitter.NewParserPool(lang),
	}
	parserRuntimes.Store(langName, rt)
	return rt.pool, rt.lang
}

func (def *langDef) processNode(node *gotreesitter.Node, out, src []byte, lines [][]byte, lang *gotreesitter.Language, functions, structs bool) []byte {
	nodeType := node.Type(lang)

	if len(def.iterateChildren) > 0 && containsStr(def.iterateChildren, nodeType) {
		for i := 0; i < node.ChildCount(); i++ {
			out = def.processNode(node.Child(i), out, src, lines, lang, functions, structs)
		}
		return out
	}

	if def.decoratedType != "" && nodeType == def.decoratedType {
		if defNode := node.ChildByFieldName(def.definitionField, lang); defNode != nil {
			node = defNode
			nodeType = node.Type(lang)
		}
	}

	switch {
	case functions && containsStr(def.funcTypes, nodeType):
		if def.funcVisible == nil || def.funcVisible(node, src, lang) {
			if def.emitFunc != nil {
				out = def.emitFunc(out, src, lines, node, lang)
			} else {
				out = langs.EmitFuncSig(out, lines, node, lang, def.indentBody, def.bodyChildType, def.singleLineSig)
			}
		}
	case structs && containsStr(def.structTypes, nodeType):
		out = def.emitStruct(out, src, lines, node, lang, functions)
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
