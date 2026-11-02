package streaming

import "testing"

func TestBuildTree_SingleRoot(t *testing.T) {
	paths := []string{
		"src/main.go",
		"src/lib/util.go",
		"src/zeta.txt",
		"src/a.md",
	}

	got := buildTree(paths)
	want := "" +
		"src/\n" +
		"├── lib/\n" +
		"│   └── util.go\n" +
		"├── a.md\n" +
		"├── main.go\n" +
		"└── zeta.txt"

	if got != want {
		t.Fatalf("buildTree mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuildTree_MultipleRoots(t *testing.T) {
	paths := []string{
		"b.txt",
		"a/one.go",
		"a/two.go",
	}

	got := buildTree(paths)
	want := "" +
		"├── a/\n" +
		"│   ├── one.go\n" +
		"│   └── two.go\n" +
		"└── b.txt"

	if got != want {
		t.Fatalf("buildTree mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuildTree_Empty(t *testing.T) {
	got := buildTree([]string{"", ".", "./"})
	if got != "" {
		t.Fatalf("buildTree should be empty, got %q", got)
	}
}

func TestBuildTree_URLPreservesScheme(t *testing.T) {
	got := buildTree([]string{"http://127.0.0.1:18082/a.txt"})
	want := "" +
		"http://127.0.0.1:18082/\n" +
		"└── a.txt"

	if got != want {
		t.Fatalf("buildTree URL mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}
