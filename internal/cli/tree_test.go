package cli

import (
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

func TestCollectTreePaths_SkipsStdinAndURL(t *testing.T) {
	cfg := lx.NewConfig()

	stdinPaths := collectTreePaths(Op{Action: "FILE", Value: "-", Type: CmdAction}, cfg, nil, nil, nil)
	if len(stdinPaths) != 0 {
		t.Fatalf("stdin should not contribute paths, got: %v", stdinPaths)
	}

	urlPaths := collectTreePaths(Op{Action: "FILE", Value: "https://example.com/repo.zip", Type: CmdAction}, cfg, nil, nil, nil)
	if len(urlPaths) != 0 {
		t.Fatalf("url should not contribute paths, got: %v", urlPaths)
	}
}

func TestPrecomputeTrees_TreeOnlyMarksFileOps(t *testing.T) {
	cfg := lx.NewConfig()
	groups := []Section{
		{
			Ops: []Op{
				{Action: "tree-only", Type: CmdAction},
				{Action: "FILE", Value: "https://example.com/repo.zip", Type: CmdAction},
			},
		},
	}

	precomputeTrees(groups, cfg, nil)
	if !groups[0].skipFileOps[1] {
		t.Fatalf("expected file op at index 1 to be skipped in tree-only section")
	}
}
