package lx

import (
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"strings"
	"testing"
)

// Every doc link in the package doc names an identifier, field or method of
// the package.
func TestDocLinksResolve(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, f := range pkgs["lx"].Files {
		files = append(files, f)
	}
	pkg, err := (&types.Config{Importer: importer.ForCompiler(fset, "source", nil)}).Check("lx", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.NewFromFiles(fset, files, "github.com/rasros/lx/pkg/lx")
	if err != nil {
		t.Fatal(err)
	}
	resolves := func(recv, name string) bool {
		if recv == "" {
			return pkg.Scope().Lookup(name) != nil
		}
		tn := pkg.Scope().Lookup(recv)
		if tn == nil {
			return false
		}
		obj, _, _ := types.LookupFieldOrMethod(tn.Type(), true, pkg, name)
		return obj != nil
	}
	var links []*comment.DocLink
	all := comment.Parser{LookupSym: func(string, string) bool { return true }}
	var walk func([]comment.Text)
	walk = func(ts []comment.Text) {
		for _, x := range ts {
			switch x := x.(type) {
			case *comment.DocLink:
				links = append(links, x)
			case *comment.Link:
				walk(x.Text)
			}
		}
	}
	for _, b := range all.Parse(p.Doc).Content {
		switch b := b.(type) {
		case *comment.Paragraph:
			walk(b.Text)
		case *comment.List:
			for _, it := range b.Items {
				for _, c := range it.Content {
					if para, ok := c.(*comment.Paragraph); ok {
						walk(para.Text)
					}
				}
			}
		}
	}
	if len(links) == 0 {
		t.Fatal("the package doc has no doc links")
	}
	for _, l := range links {
		if l.ImportPath == "" && !resolves(l.Recv, l.Name) {
			t.Errorf("doc link [%s.%s] does not resolve", l.Recv, l.Name)
		}
	}
}
