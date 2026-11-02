package skeleton

import (
	"slices"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/rasros/lx/pkg/lx/skeleton/langs"
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

	c := &langs.Ctx{
		Src:       src,
		Lines:     lines,
		Lang:      lang,
		Comments:  def.comments,
		Functions: functions,
		Structs:   structs,
	}
	root := tree.RootNode()
	for i := 0; i < root.ChildCount(); i++ {
		out = def.processNode(c, out, root.Child(i))
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

func (def *langDef) processNode(c *langs.Ctx, out []internal.LineRange, node *gotreesitter.Node) []internal.LineRange {
	nodeType := c.Type(node)

	if slices.Contains(def.iterateChildren, nodeType) {
		for i := 0; i < node.ChildCount(); i++ {
			out = def.processNode(c, out, node.Child(i))
		}
		return out
	}

	if slices.Contains(def.frames, nodeType) {
		return def.processFrame(c, out, node)
	}

	// A namespace without a declaration list (extern "C" int f(void);)
	// wraps a single declaration, left to unwrap.
	if slices.Contains(def.namespaces, nodeType) {
		if body := node.ChildByFieldName("body", c.Lang); body != nil && c.Type(body) == "declaration_list" {
			return def.processNamespace(c, out, node, body)
		}
	}

	outerStart := int(node.StartPoint().Row)

	if def.decoratedType != "" && nodeType == def.decoratedType {
		if defNode := node.ChildByFieldName(def.definitionField, c.Lang); defNode != nil {
			node = defNode
			nodeType = c.Type(node)
		}
	}
	if def.unwrap != nil {
		node = def.unwrap(c, node)
		nodeType = c.Type(node)
	}
	sigStart := int(node.StartPoint().Row)

	switch {
	case c.Functions && slices.Contains(def.funcTypes, nodeType):
		if def.funcVisible == nil || def.funcVisible(node, c.Src, c.Lang) {
			out = c.EmitLeadingDoc(out, outerStart)
			out = langs.EmitDecorators(out, c.Lines, outerStart, sigStart)
			if def.emitFunc != nil {
				out = def.emitFunc(c, out, node)
			} else {
				out = langs.EmitFuncSig(out, c.Lines, node, c.Lang, def.indentBody, def.bodyChildType)
				if def.indentBody {
					out = langs.AppendPyDocstring(out, c.Lines, node, c.Lang)
				}
			}
		}
	case slices.Contains(def.structTypes, nodeType):
		// Types are walked with either flag: with functions alone they keep
		// the methods they hold. Their doc and decorators go only with a
		// type that keeps something.
		if typ := def.emitType(c, node); len(typ) > 0 {
			out = c.EmitLeadingDoc(out, outerStart)
			out = langs.EmitDecorators(out, c.Lines, outerStart, sigStart)
			out = append(out, typ...)
		}
	}
	return out
}

// processNamespace walks the declarations in the body of a namespace like
// top-level ones. The namespace's doc, header and closing row are kept
// around them for context when any of them keeps something.
func (def *langDef) processNamespace(c *langs.Ctx, out []internal.LineRange, n, body *gotreesitter.Node) []internal.LineRange {
	var inner []internal.LineRange
	for i := 0; i < body.ChildCount(); i++ {
		inner = def.processNode(c, inner, body.Child(i))
	}
	if len(inner) == 0 {
		return out
	}
	start, headerEnd := int(n.StartPoint().Row), int(body.StartPoint().Row)
	out = c.EmitLeadingDoc(out, start)
	out = internal.KeepLines(out, c.Lines, start, headerEnd)
	out = append(out, inner...)
	if closingRow := int(body.EndPoint().Row); closingRow > headerEnd {
		out = internal.KeepLine(out, c.Lines, closingRow)
	}
	return out
}

// processFrame walks the children of a preprocessor conditional like
// top-level declarations. Its first row (#if, #ifdef, #else) and, when it ends
// in a token, its last (#endif) are kept around them when any of them keeps
// something. An #elif or #else is a child frame of its #if.
func (def *langDef) processFrame(c *langs.Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	var inner []internal.LineRange
	for i := 0; i < n.ChildCount(); i++ {
		inner = def.processNode(c, inner, n.Child(i))
	}
	if len(inner) == 0 {
		return out
	}
	first, last := int(n.StartPoint().Row), int(n.EndPoint().Row)
	out = internal.KeepLine(out, c.Lines, first)
	out = append(out, inner...)
	if k := n.ChildCount(); k > 0 && !n.Child(k-1).IsNamed() && last > first {
		out = internal.KeepLine(out, c.Lines, last)
	}
	return out
}

// emitType returns the rows kept of the type declaration n.
func (def *langDef) emitType(c *langs.Ctx, n *gotreesitter.Node) []internal.LineRange {
	switch {
	case def.emitStruct != nil:
		return def.emitStruct(c, nil, n)
	case def.members != nil:
		return def.members.Emit(c, nil, n)
	case c.Structs:
		return c.KeepNode(nil, n)
	}
	return nil
}
