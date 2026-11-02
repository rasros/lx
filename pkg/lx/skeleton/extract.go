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

// extract returns the rows of src (split into lines) that make up its
// skeleton, unmerged, or ok=false when src cannot be parsed.
func extract(langName string, src []byte, lines [][]byte, functions, structs bool) (out []internal.LineRange, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = nil, false
		}
	}()

	def, found := langDefs[langName]
	if !found {
		return nil, false
	}
	pool, lang := parserPoolFor(langName, def.newLang)

	tree, err := pool.Parse(src)
	if err != nil {
		return nil, false
	}
	defer tree.Release()

	root := tree.RootNode()

	for i := 0; i < root.ChildCount(); i++ {
		out = def.processNode(root.Child(i), out, src, lines, lang, functions, structs)
	}
	return out, true
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

func (def *langDef) processNode(node *gotreesitter.Node, out []internal.LineRange, src []byte, lines [][]byte, lang *gotreesitter.Language, functions, structs bool) []internal.LineRange {
	nodeType := node.Type(lang)

	if len(def.iterateChildren) > 0 && containsStr(def.iterateChildren, nodeType) {
		for i := 0; i < node.ChildCount(); i++ {
			out = def.processNode(node.Child(i), out, src, lines, lang, functions, structs)
		}
		return out
	}

	outerStart := int(node.StartPoint().Row)

	if def.decoratedType != "" && nodeType == def.decoratedType {
		if defNode := node.ChildByFieldName(def.definitionField, lang); defNode != nil {
			node = defNode
			nodeType = node.Type(lang)
		}
	}

	switch {
	case functions && containsStr(def.funcTypes, nodeType):
		if def.funcVisible == nil || def.funcVisible(node, src, lang) {
			out = prependDoc(out, lines, outerStart)
			out = langs.EmitDecorators(out, lines, outerStart, int(node.StartPoint().Row))
			if def.emitFunc != nil {
				out = def.emitFunc(out, src, lines, node, lang)
			} else {
				out = langs.EmitFuncSig(out, lines, node, lang, def.indentBody, def.bodyChildType, def.singleLineSig)
				if def.indentBody {
					out = langs.AppendPyDocstring(out, lines, node, lang)
				}
			}
		}
	case structs && containsStr(def.structTypes, nodeType):
		out = prependDoc(out, lines, outerStart)
		out = langs.EmitDecorators(out, lines, outerStart, int(node.StartPoint().Row))
		out = def.emitStruct(out, src, lines, node, lang, functions)
	}
	return out
}

func prependDoc(out []internal.LineRange, lines [][]byte, startRow int) []internal.LineRange {
	return langs.EmitLeadingDoc(out, lines, startRow)
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
