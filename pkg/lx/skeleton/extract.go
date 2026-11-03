package skeleton

import (
	"bytes"
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
// skeleton, unmerged, or ok=false when src cannot be parsed. A C header
// (header) may be C++: see extractHeader.
func extract(langName string, header bool, src []byte, lines [][]byte, functions, structs bool) (out []internal.LineRange, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = nil, false
		}
	}()

	if langName == "c" && header {
		return extractHeader(src, lines, functions, structs)
	}
	p, ok := parse(langName, src)
	if !ok {
		return nil, false
	}
	defer p.tree.Release()
	return p.walk(src, lines, functions, structs), true
}

// parsed is src parsed with one language's grammar.
type parsed struct {
	def  *langDef
	lang *gotreesitter.Language
	tree *gotreesitter.Tree
}

func parse(langName string, src []byte) (parsed, bool) {
	def, found := langDefs[langName]
	if !found {
		return parsed{}, false
	}
	pool, lang := parserPoolFor(langName, def.newLang)
	tree, err := pool.Parse(src)
	if err != nil {
		return parsed{}, false
	}
	return parsed{def: &def, lang: lang, tree: tree}, true
}

// walk returns the rows of the skeleton of the parsed src, unmerged.
func (p parsed) walk(src []byte, lines [][]byte, functions, structs bool) []internal.LineRange {
	c := &langs.Ctx{
		Src:       src,
		Lines:     lines,
		Lang:      p.lang,
		Comments:  p.def.comments,
		Functions: functions,
		Structs:   structs,
	}
	var out []internal.LineRange
	root := p.tree.RootNode()
	for i := 0; i < root.ChildCount(); i++ {
		out = p.def.processNode(c, out, root.Child(i))
	}
	if p.def.conditionals {
		hidden, trailing := p.comments(lines)
		out = langs.BalanceConditionals(lines, out, hidden)
		out = cutTrailingComments(internal.MergeRanges(out), trailing)
	}
	return out
}

// cutTrailingComments cuts a range's last row before a multi-row comment
// that starts after code, so the skeleton shows no unclosed comment.
func cutTrailingComments(ranges []internal.LineRange, trailing map[int]int) []internal.LineRange {
	for i, r := range ranges {
		if col, ok := trailing[r.End-1]; ok && r.Cut == 0 {
			ranges[i].Cut = col
		}
	}
	return ranges
}

// comments reports the continuation rows of multi-row comments and raw
// strings, where a directive-like line is none, and, by row, the column
// where a multi-row comment starts after code.
func (p parsed) comments(lines [][]byte) (hidden []bool, trailing map[int]int) {
	hidden = make([]bool, len(lines))
	trailing = make(map[int]int)
	var walk func(n *gotreesitter.Node)
	walk = func(n *gotreesitter.Node) {
		first, last := int(n.StartPoint().Row), int(n.EndPoint().Row)
		if first == last {
			return
		}
		switch t := n.Type(p.lang); t {
		case "comment", "raw_string_literal":
			for row := first + 1; row <= last && row < len(hidden); row++ {
				hidden[row] = true
			}
			if t == "comment" && first < len(lines) {
				col := int(n.StartPoint().Column)
				if code := bytes.TrimRight(lines[first][:min(col, len(lines[first]))], " \t"); len(bytes.TrimSpace(code)) > 0 {
					trailing[first] = len(code)
				}
			}
			return
		}
		for i := 0; i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(p.tree.RootNode())
	return hidden, trailing
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

	if def.recoverErrors {
		switch {
		case node.IsError() || hidden(c.Lang, node):
			return def.processError(c, out, node)
		case langs.Misrecovered(c, node):
			return out
		}
	}

	if slices.Contains(def.iterateChildren, nodeType) {
		for i := 0; i < node.ChildCount(); i++ {
			out = def.processNode(c, out, node.Child(i))
		}
		return out
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

	// A node of both funcTypes and structTypes (a C declaration) that is no
	// function is a type.
	switch {
	case c.Functions && slices.Contains(def.funcTypes, nodeType) && (def.funcVisible == nil || def.funcVisible(c, node)):
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
	case slices.Contains(def.structTypes, nodeType):
		// Types are walked with either flag: with functions alone they keep
		// their methods.
		if typ := def.emitType(c, node); len(typ) > 0 {
			out = c.EmitLeadingDoc(out, outerStart)
			out = langs.EmitDecorators(out, c.Lines, outerStart, sigStart)
			out = append(out, typ...)
		}
	}
	return out
}

// processNamespace walks a namespace body like top-level declarations,
// keeping the namespace's header and closing row when anything is kept.
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

// processError walks the children of an error node, or of a node the
// grammar hides, like top-level declarations. A C header's
// `#ifdef __cplusplus extern "C" { #endif` guard leaves one.
func (def *langDef) processError(c *langs.Ctx, out []internal.LineRange, n *gotreesitter.Node) []internal.LineRange {
	for i := 0; i < n.ChildCount(); i++ {
		out = def.processNode(c, out, n.Child(i))
	}
	return out
}

// hidden reports a node of a symbol the grammar keeps out of trees.
func hidden(lang *gotreesitter.Language, n *gotreesitter.Node) bool {
	sym := int(n.Symbol())
	if sym >= len(lang.SymbolMetadata) || n.IsError() {
		return false
	}
	md := lang.SymbolMetadata[sym]
	return !md.Visible || md.Supertype || md.GeneratedRepeatAux
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
