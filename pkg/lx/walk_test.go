package lx

import (
	"context"
	"testing"
	"testing/fstest"
)

func TestWalker_WithFilters(t *testing.T) {
	mockFS := fstest.MapFS{
		"main.go":   {Data: []byte("package main")},
		"README.md": {Data: []byte("# Docs")},
	}

	w := NewWalker(WalkerOptions{
		FS:       mockFS,
		Includes: []string{"*.go"},
	})

	ctx := context.Background()
	// Use empty string to represent the root of the fs
	ch := w.Walk(ctx, []string{""})

	var found []string
	for f := range ch {
		found = append(found, f.Path)
	}

	if len(found) != 1 || found[0] != "main.go" {
		t.Errorf("Filter failed, found: %v (expected [main.go])", found)
	}
}
