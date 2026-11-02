package cli

import (
	"context"
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

// Stdin is left to the content pass, and a missing path is counted so the
// stats summary can report it.
func TestResolveSections_StdinAndMissingPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	sections := []Section{{Ops: []Op{
		{Action: "FILE", Value: "-", Type: CmdAction},
		{Action: "FILE", Value: "nope.go", Type: CmdAction},
	}}}
	if got := resolveSections(context.Background(), sections, lx.NewCollector()); got != 1 {
		t.Errorf("unresolved = %d, want 1", got)
	}
	if len(sections[0].resolved) != 0 {
		t.Errorf("resolved %v, want nothing", sections[0].resolved)
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
