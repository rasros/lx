package walker

import (
	"io/fs"
	"reflect"
	"sort"
	"testing"
	"testing/fstest"
)

// Entries keeps the files Walk finds in an fs.FS of the same tree, whatever
// order the entries come in and whether directories have entries.
func TestEntries_AgreesWithWalk(t *testing.T) {
	files := []string{
		"a.go", "a/x.go", "a/b/y.go", "a/b/z.txt", ".hidden/h.go", "a/.h.go",
		"docs/readme.md", "docs/deep/more.md", "src/main.go", "src/vendor/v.go",
	}
	fsys := fstest.MapFS{}
	for _, f := range files {
		fsys[f] = &fstest.MapFile{Data: []byte("x")}
	}
	cases := [][]Override{
		nil,
		{IncludeOverride("*.go")},
		{ExcludeOverride("a/b/")},
		{IncludeOverride("pre/a/**"), ExcludeOverride("pre/a/b/y.go")},
		{IncludeOverride("pre/docs/**"), IncludeOverride("!*.md"), IncludeOverride("pre/docs/readme.md")},
		{ExcludeOverride("vendor"), ExcludeOverride("!src/vendor")},
		{IncludeOverride("src/*.go")},
	}
	for _, overrides := range cases {
		for _, hidden := range []bool{false, true} {
			opts := Options{Overrides: overrides, OverridePrefix: "pre/", SkipHidden: !hidden}
			var walked []string
			err := New(opts).Walk(fsys, ".", func(p string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					walked = append(walked, p)
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			// Reversed, so a file comes before its directories.
			var kept []string
			e := New(opts).Entries()
			for i := len(files) - 1; i >= 0; i-- {
				if e.Keep(files[i], false) {
					kept = append(kept, files[i])
				}
			}
			sort.Strings(walked)
			sort.Strings(kept)
			if !reflect.DeepEqual(walked, kept) {
				t.Errorf("overrides %v, hidden %v: Entries kept %v, Walk found %v", overrides, hidden, kept, walked)
			}
		}
	}
}

// A skipped directory is reported once, and the entries below it not at
// all, as a walk reports them.
func TestEntries_ReportsSkippedDirOnce(t *testing.T) {
	var reported []string
	e := New(Options{
		Overrides: []Override{ExcludeOverride("gen/")},
		OnIgnore:  func(p, _ string) { reported = append(reported, p) },
	}).Entries()
	for _, p := range []string{"gen/a.go", "gen/b/c.go", "keep.go"} {
		e.Keep(p, false)
	}
	if e.Keep("gen", true) {
		t.Error("gen kept")
	}
	if want := []string{"gen"}; !reflect.DeepEqual(reported, want) {
		t.Errorf("reported %v, want %v", reported, want)
	}
}
