package discover

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rasros/lx/pkg/lx/sources"
)

func paths(files []sources.InputFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// A URL is one input, fetched only when rendered.
func TestCollect_URL(t *testing.T) {
	url := "https://example.com/repo.zip"
	files, failures, err := NewCollector("", nil).Collect(context.Background(), url, Options{})
	if err != nil || len(failures) > 0 {
		t.Fatal(err, failures)
	}
	if got := paths(files); !slices.Equal(got, []string{url}) {
		t.Fatalf("got %v, want [%s]", got, url)
	}
}

// A walk stops when its context is done.
func TestCollect_CancelledWalk(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files, _, err := NewCollector("", nil).Collect(ctx, dir, Options{})
	if !errors.Is(err, context.Canceled) || len(files) != 0 {
		t.Fatalf("got %v, %v; want context.Canceled and no inputs", paths(files), err)
	}
}
