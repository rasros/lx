package lx

import (
	"go/ast"
	"go/doc/comment"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// checkPackage parses and type-checks the package's own files, its
// dependencies imported from the export data the go command builds, and
// returns its doc comment and its types.
func checkPackage(t *testing.T) (string, *types.Package) {
	t.Helper()
	out, err := exec.Command("go", "list", "-export", "-deps", "-f", "{{.ImportPath}}\t{{.Export}}", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	exports := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path, file, ok := strings.Cut(line, "\t"); ok {
			exports[path] = file
		}
	}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	var docText string
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
		files = append(files, f)
	}
	if docText == "" {
		t.Fatal("no package doc comment")
	}

	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		return os.Open(exports[path])
	})
	conf := types.Config{Importer: imp}
	pkg, err := conf.Check("github.com/rasros/lx/pkg/lx", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	return docText, pkg
}

// docParts returns the doc links of the package doc, the text outside them,
// and its code blocks.
func docParts(text string) (links []*comment.DocLink, plain []string, code []string) {
	// Every bracketed name parses as a link, for the test to judge.
	p := comment.Parser{LookupSym: func(recv, name string) bool { return true }}
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
	for _, b := range p.Parse(text).Content {
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

// typeName returns the exported type of pkg called name, or nil.
func typeName(pkg *types.Package, name string) *types.TypeName {
	tn, _ := pkg.Scope().Lookup(name).(*types.TypeName)
	if tn == nil || !tn.Exported() {
		return nil
	}
	return tn
}

// hasMember reports whether the type named by tn has an exported field or
// method called name.
func hasMember(pkg *types.Package, tn *types.TypeName, name string) bool {
	obj, _, _ := types.LookupFieldOrMethod(tn.Type(), true, pkg, name)
	return obj != nil && obj.Exported()
}

// apiNames are the package's exported names and the exported fields and
// methods of its exported types.
func apiNames(pkg *types.Package) map[string]bool {
	names := map[string]bool{}
	scope := pkg.Scope()
	for _, n := range scope.Names() {
		obj := scope.Lookup(n)
		if !obj.Exported() {
			continue
		}
		names[n] = true
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		ms := types.NewMethodSet(types.NewPointer(tn.Type()))
		for i := 0; i < ms.Len(); i++ {
			if m := ms.At(i).Obj(); m.Exported() {
				names[m.Name()] = true
			}
		}
		if st, ok := tn.Type().Underlying().(*types.Struct); ok {
			for i := 0; i < st.NumFields(); i++ {
				if f := st.Field(i); f.Exported() {
					names[f.Name()] = true
				}
			}
		}
	}
	return names
}

// distance is the Levenshtein distance between a and b.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// misspelled returns the exported name word is a near miss of, or "". Only a
// capitalized word of five letters or more is judged, so English and brand
// names far from every identifier pass, and a plural of a name is not a miss.
func misspelled(word string, names map[string]bool) string {
	if names[word] || len(word) < 5 || !unicode.IsUpper(rune(word[0])) {
		return ""
	}
	for n := range names {
		if word == n+"s" {
			return ""
		}
	}
	limit := 1
	if len(word) >= 10 {
		limit = 2
	}
	for n := range names {
		if distance(word, n) <= limit {
			return n
		}
	}
	return ""
}

var (
	selectorRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b`)
	wordRe     = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\b`)
)

// Every identifier of this package the package doc names exists. A doc link
// must name an exported identifier, or an exported field or method of an
// exported type. In the text outside links, a selector of a package type
// must name an exported member of it, and a word close to an exported name
// without being one is taken for a misspelling of it.
func TestDocNamesExist(t *testing.T) {
	text, pkg := checkPackage(t)
	links, plain, _ := docParts(text)
	if len(links) == 0 {
		t.Fatal("the package doc has no doc links")
	}

	for _, l := range links {
		if l.ImportPath != "" {
			continue
		}
		if l.Recv == "" {
			if obj := pkg.Scope().Lookup(l.Name); obj == nil || !obj.Exported() {
				t.Errorf("[%s] is not exported by the package", l.Name)
			}
			continue
		}
		tn := typeName(pkg, l.Recv)
		if tn == nil {
			t.Errorf("[%s.%s]: %s is not an exported type", l.Recv, l.Name, l.Recv)
			continue
		}
		if !hasMember(pkg, tn, l.Name) {
			t.Errorf("[%s.%s]: %s has no exported field or method %s", l.Recv, l.Name, l.Recv, l.Name)
		}
	}

	names := apiNames(pkg)
	for _, s := range plain {
		for _, m := range selectorRe.FindAllStringSubmatch(s, -1) {
			if tn := typeName(pkg, m[1]); tn != nil && !hasMember(pkg, tn, m[2]) {
				t.Errorf("%s: %s has no exported field or method %s", m[0], m[1], m[2])
			}
		}
		for _, w := range wordRe.FindAllString(selectorRe.ReplaceAllString(s, " "), -1) {
			if n := misspelled(w, names); n != "" {
				t.Errorf("%s is not an exported name; is it %s?", w, n)
			}
		}
	}
}

// Misspellings are judged against the package's names alone, so brand names
// and abbreviations pass.
func TestMisspelled(t *testing.T) {
	names := map[string]bool{"Execute": true, "Stream": true, "ExpandArchives": true}
	for word, want := range map[string]string{
		"Execut":        "Execute",
		"Executes":      "",
		"Streams":       "",
		"Stream":        "",
		"ExpandArchive": "ExpandArchives",
		"GitHub":        "",
		"JavaScript":    "",
		"PowerPoint":    "",
		"U":             "",
		"execut":        "",
	} {
		got := misspelled(word, names)
		if got != want {
			t.Errorf("misspelled(%q) = %q, want %q", word, got, want)
		}
	}
}

// The package doc's example is Example's body, which compiles.
func TestDocExampleCompiles(t *testing.T) {
	text, _ := checkPackage(t)
	_, _, code := docParts(text)
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
