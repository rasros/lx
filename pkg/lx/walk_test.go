package lx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWalker_WithFilters(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Docs"), 0644)

	w := NewWalker(WalkerOptions{})

	// Test: Filter only .go files
	goFilter := func(f InputFile) bool {
		return filepath.Ext(f.Path) == ".go"
	}

	ctx := context.Background()
	ch := w.Walk(ctx, []string{tmpDir}, goFilter)

	var found []string
	for f := range ch {
		found = append(found, filepath.Base(f.Path))
	}

	if len(found) != 1 || found[0] != "main.go" {
		t.Errorf("Filter failed, found: %v", found)
	}
}
