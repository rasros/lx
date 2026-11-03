package lx

import (
	"go/ast"
	"go/doc/comment"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// apiTypes are the package's exported types, so the doc links naming their
// fields and methods can be checked. TestDocLinksExist fails when a type is
// missing here, so the list cannot fall behind the package.
var apiTypes = map[string]reflect.Type{
	"Config":          reflect.TypeFor[Config](),
	"RenderOptions":   reflect.TypeFor[RenderOptions](),
	"Plan":            reflect.TypeFor[Plan](),
	"Report":          reflect.TypeFor[Report](),
	"Tokenizer":       reflect.TypeFor[Tokenizer](),
	"FileContext":     reflect.TypeFor[FileContext](),
	"SectionContext":  reflect.TypeFor[SectionContext](),
	"PromptContext":   reflect.TypeFor[PromptContext](),
	"TreeContext":     reflect.TypeFor[TreeContext](),
	"MetaContext":     reflect.TypeFor[MetaContext](),
	"HeaderContext":   reflect.TypeFor[HeaderContext](),
	"FooterContext":   reflect.TypeFor[FooterContext](),
	"InputFile":       reflect.TypeFor[InputFile](),
	"DiscoverOptions": reflect.TypeFor[DiscoverOptions](),
	"Glob":            reflect.TypeFor[Glob](),
	"GlobKind":        reflect.TypeFor[GlobKind](),
	"Collector":       reflect.TypeFor[Collector](),
	"URLError":        reflect.TypeFor[URLError](),
	"Failure":         reflect.TypeFor[Failure](),
	"FailureKind":     reflect.TypeFor[FailureKind](),
	"FailureHandler":  reflect.TypeFor[FailureHandler](),
	"Stream":          reflect.TypeFor[Stream](),
}

// parsePackage returns the package's doc comment and its exported top-level
// names, with whether each is a type.
func parsePackage(t *testing.T) (string, map[string]bool) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var docText string
	names := map[string]bool{}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if f.Doc != nil {
			docText += f.Doc.Text()
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					names[d.Name.Name] = false
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							names[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, id := range s.Names {
							if id.IsExported() {
								names[id.Name] = false
							}
						}
					}
				}
			}
		}
	}
	if docText == "" {
		t.Fatal("no package doc comment")
	}
	return docText, names
}

// member reports whether typ has a field or method called name.
func member(typ reflect.Type, name string) bool {
	if typ.Kind() == reflect.Interface {
		_, ok := typ.MethodByName(name)
		return ok
	}
	if _, ok := reflect.PointerTo(typ).MethodByName(name); ok {
		return true
	}
	if typ.Kind() == reflect.Struct {
		f, ok := typ.FieldByName(name)
		return ok && f.IsExported()
	}
	return false
}

// docLinks returns the doc links of the package doc and the words of its
// text outside them, its code blocks apart.
func docLinks(doc *comment.Doc) (links []*comment.DocLink, plain []string, code []string) {
	var walk func([]comment.Text)
	walk = func(ts []comment.Text) {
		for _, t := range ts {
			switch t := t.(type) {
			case comment.Plain:
				plain = append(plain, string(t))
			case comment.Italic:
				plain = append(plain, string(t))
			case *comment.DocLink:
				links = append(links, t)
			case *comment.Link:
				walk(t.Text)
			}
		}
	}
	for _, b := range doc.Content {
		switch b := b.(type) {
		case *comment.Paragraph:
			walk(b.Text)
		case *comment.Heading:
			walk(b.Text)
		case *comment.List:
			for _, it := range b.Items {
				for _, c := range it.Content {
					if p, ok := c.(*comment.Paragraph); ok {
						walk(p.Text)
					}
				}
			}
		case *comment.Code:
			code = append(code, b.Text)
		}
	}
	return links, plain, code
}

// Every identifier the package doc names must exist: a doc link must name
// an exported identifier, or a field or method of an exported type, and an
// identifier outside a link, which godoc would not check, is an error.
func TestDocLinksExist(t *testing.T) {
	text, names := parsePackage(t)

	for n, isType := range names {
		if isType {
			if _, ok := apiTypes[n]; !ok {
				t.Errorf("exported type %s is missing from apiTypes", n)
			}
		}
	}
	for n := range apiTypes {
		if !names[n] {
			t.Errorf("apiTypes has %s, which is not an exported type", n)
		}
	}

	// Every bracketed name parses as a link, for the check below to judge.
	p := comment.Parser{LookupSym: func(recv, name string) bool { return true }}
	links, plain, _ := docLinks(p.Parse(text))
	if len(links) == 0 {
		t.Fatal("the package doc has no doc links")
	}
	for _, l := range links {
		if l.ImportPath != "" {
			continue
		}
		name := l.Name
		if l.Recv != "" {
			typ, ok := apiTypes[l.Recv]
			if !ok {
				t.Errorf("[%s.%s]: %s is not an exported type", l.Recv, name, l.Recv)
				continue
			}
			if !member(typ, name) {
				t.Errorf("[%s.%s]: %s has no exported field or method %s", l.Recv, name, l.Recv, name)
			}
			continue
		}
		if _, ok := names[name]; !ok {
			t.Errorf("[%s] is not exported by the package", name)
		}
	}

	// A word with an inner capital, or a selector of capitalized words, is an
	// identifier, which must be a link so the check above sees it.
	ident := regexp.MustCompile(`\b[A-Z][A-Za-z0-9]*\.[A-Z][A-Za-z0-9]*\b|\b[A-Z][a-z0-9]+[A-Z][A-Za-z0-9]*\b`)
	for _, s := range plain {
		for _, m := range ident.FindAllString(s, -1) {
			t.Errorf("%s is not a doc link; write it as [%s]", m, m)
		}
	}
}

// The package doc's example is Example's body, which compiles.
func TestDocExampleCompiles(t *testing.T) {
	text, _ := parsePackage(t)
	var p comment.Parser
	_, _, code := docLinks(p.Parse(text))
	var doc string
	for _, c := range code {
		const start = "func main() {\n"
		if i := strings.Index(c, start); i >= 0 {
			body := c[i+len(start):]
			doc = strings.TrimSuffix(strings.TrimRight(body, "\n"), "}")
		}
	}
	if doc == "" {
		t.Fatal("the package doc has no example main function")
	}

	fset := token.NewFileSet()
	src, err := os.ReadFile("example_test.go")
	if err != nil {
		t.Fatal(err)
	}
	f, err := parser.ParseFile(fset, "example_test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var example string
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "Example" {
			lbrace := fset.Position(fn.Body.Lbrace).Offset
			rbrace := fset.Position(fn.Body.Rbrace).Offset
			example = string(src[lbrace+len("{\n") : rbrace])
		}
	}
	if example == "" {
		t.Fatal("example_test.go has no Example function")
	}
	if doc != example {
		t.Errorf("the package doc's main differs from Example:\n%s\nExample:\n%s", doc, example)
	}
}
