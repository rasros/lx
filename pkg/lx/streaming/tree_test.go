package streaming

import "testing"

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
