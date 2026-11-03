package discover

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/walker"
)

// countingHost is the OS seen from a fixed working directory, counting the
// ignore files read and the .git entries stat'ed.
type countingHost struct {
	walker.OSHost
	cwd string

	mu    sync.Mutex
	reads map[string]int
}

func (h *countingHost) Getwd() (string, error) { return h.cwd, nil }

func (h *countingHost) ReadFile(name string) ([]byte, error) {
	h.count(name)
	return os.ReadFile(name)
}

func (h *countingHost) Stat(name string) (fs.FileInfo, error) {
	h.count(name)
	return os.Stat(name)
}

func (h *countingHost) count(name string) {
	switch filepath.Base(name) {
	case ".gitignore", ".ignore", ".lxignore", ".git":
		h.mu.Lock()
		h.reads[name]++
		h.mu.Unlock()
	}
}

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
