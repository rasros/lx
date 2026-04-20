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

func TestCollectTreePaths_StdinAndURLHandling(t *testing.T) {
	ctx := context.Background()
	runCfg := lx.RunnerConfig{}

	stdinPaths := collectTreePaths(ctx, Op{Action: "FILE", Value: "-", Type: CmdAction}, runCfg, nil, nil, nil)
	if len(stdinPaths) != 0 {
		t.Fatalf("stdin should not contribute paths, got: %v", stdinPaths)
	}

	url := "https://example.com/repo.zip"
	urlPaths := collectTreePaths(ctx, Op{Action: "FILE", Value: url, Type: CmdAction}, runCfg, nil, nil, nil)
	if len(urlPaths) != 1 || urlPaths[0] != url {
		t.Fatalf("url should be included as a file when not expanded, got: %v", urlPaths)
	}
}

func TestCollectTreePaths_ExpandsLocalArchiveWithEntryIncludes(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	t.Chdir(tmp)

	archivePath := filepath.Join(tmp, "archive.zip")
	createTestZip(t, archivePath, map[string]string{
		"hello.txt":       "hello\n",
		"nested/world.go": "package nested\n",
	})

	paths := collectTreePaths(ctx, Op{Action: "FILE", Value: "archive.zip", Type: CmdAction}, lx.RunnerConfig{ExpandArchives: true}, []string{"*.go"}, nil, nil)
	want := []string{"archive.zip/nested/world.go"}
	if !slices.Equal(paths, want) {
		t.Fatalf("collectTreePaths archive include mismatch\nwant: %v\ngot:  %v", want, paths)
	}
}

func TestPrecomputeTrees_TreeOnlyMarksFileOps(t *testing.T) {
	ctx := context.Background()
	groups := []Section{
		{
			Ops: []Op{
				{Action: "tree-only", Type: CmdAction},
				{Action: "FILE", Value: "https://example.com/repo.zip", Type: CmdAction},
			},
		},
	}

	precomputeTrees(ctx, groups, nil)
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
