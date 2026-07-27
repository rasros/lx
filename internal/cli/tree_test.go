package cli

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

func TestBuildASCIITree_SingleRoot(t *testing.T) {
	paths := []string{
		"src/main.go",
		"src/lib/util.go",
		"src/zeta.txt",
		"src/a.md",
	}

	got := buildASCIITree(paths)
	want := "" +
		"src/\n" +
		"├── lib/\n" +
		"│   └── util.go\n" +
		"├── a.md\n" +
		"├── main.go\n" +
		"└── zeta.txt"

	if got != want {
		t.Fatalf("buildASCIITree mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuildASCIITree_MultipleRoots(t *testing.T) {
	paths := []string{
		"b.txt",
		"a/one.go",
		"a/two.go",
	}

	got := buildASCIITree(paths)
	want := "" +
		"├── a/\n" +
		"│   ├── one.go\n" +
		"│   └── two.go\n" +
		"└── b.txt"

	if got != want {
		t.Fatalf("buildASCIITree mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuildASCIITree_Empty(t *testing.T) {
	got := buildASCIITree([]string{"", ".", "./"})
	if got != "" {
		t.Fatalf("buildASCIITree should be empty, got %q", got)
	}
}

func TestBuildASCIITree_URLPreservesScheme(t *testing.T) {
	got := buildASCIITree([]string{"http://127.0.0.1:18082/a.txt"})
	want := "" +
		"http://127.0.0.1:18082/\n" +
		"└── a.txt"

	if got != want {
		t.Fatalf("buildASCIITree URL mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func testResolveContext(section Section) resolveContext {
	var cleanups []func()
	return resolveContext{section: section, cleanups: &cleanups}
}

func resolvedPaths(files []lx.InputFile) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	return paths
}

func TestResolveOp_StdinAndURLHandling(t *testing.T) {
	ctx := context.Background()
	rc := testResolveContext(Section{})

	stdin, _ := resolveOp(ctx, Op{Action: "FILE", Value: "-", Type: CmdAction}, rc)
	if len(stdin) != 0 {
		t.Fatalf("stdin should not resolve to inputs, got: %v", resolvedPaths(stdin))
	}

	url := "https://example.com/repo.zip"
	urls, _ := resolveOp(ctx, Op{Action: "FILE", Value: url, Type: CmdAction}, rc)
	if got := resolvedPaths(urls); len(got) != 1 || got[0] != url {
		t.Fatalf("url should resolve to one input, got: %v", got)
	}
}

func TestResolveOp_ExpandsLocalArchiveWithEntryIncludes(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	t.Chdir(tmp)

	createTestZip(t, filepath.Join(tmp, "archive.zip"), map[string]string{
		"hello.txt":       "hello\n",
		"nested/world.go": "package nested\n",
	})

	rc := testResolveContext(Section{
		RunCfg:   lx.RunnerConfig{ExpandArchives: true},
		Includes: []string{"*.go"},
	})
	files, _ := resolveOp(ctx, Op{Action: "FILE", Value: "archive.zip", Type: CmdAction}, rc)

	want := []string{"archive.zip/nested/world.go"}
	if got := resolvedPaths(files); !slices.Equal(got, want) {
		t.Fatalf("archive include mismatch\nwant: %v\ngot:  %v", want, got)
	}
}

// A missing path is reported so the stats summary can count it.
func TestResolveOp_ReportsUnresolvedPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	files, unresolved := resolveOp(context.Background(),
		Op{Action: "FILE", Value: "nope.go", Type: CmdAction}, testResolveContext(Section{}))

	if len(files) != 0 {
		t.Errorf("got %v, want no inputs", resolvedPaths(files))
	}
	if unresolved != 1 {
		t.Errorf("unresolved = %d, want 1", unresolved)
	}
}

func TestPrecomputeTrees_TreeOnlyMarksFileOps(t *testing.T) {
	groups := []Section{
		{
			Ops: []Op{
				{Action: "tree-only", Type: CmdAction},
				{Action: "FILE", Value: "https://example.com/repo.zip", Type: CmdAction},
			},
		},
	}

	precomputeTrees(groups)
	if !groups[0].skipFileOps[1] {
		t.Fatalf("expected file op at index 1 to be skipped in tree-only section")
	}
}

func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
}
