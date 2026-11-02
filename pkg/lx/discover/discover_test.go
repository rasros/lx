package discover

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// Named files are never filtered, so a piped file list reads no ignore
// files at all. Named directories share the collector's rule cache, so each
// directory's ignore files are read once however many walks reach it.
func TestCollect_IgnoreFileReads(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"objects", "refs"} {
		if err := os.MkdirAll(filepath.Join(dir, ".git", sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".git/HEAD", "ref: refs/heads/main\n")
	write(".gitignore", "*.log\n")

	var fileTargets, dirTargets []string
	for _, d := range []string{"a/b/c", "a/b/d", "e/f/g"} {
		for i := range 20 {
			rel := d + "/f" + strings.Repeat("x", i) + ".go"
			write(rel, "package x")
			fileTargets = append(fileTargets, filepath.Join(dir, filepath.FromSlash(rel)))
		}
		write(d+"/skip.log", "ignored in a walk")
	}
	fileTargets = append(fileTargets, filepath.Join(dir, "a", "b", "c", "skip.log"))
	for _, d := range []string{".", "a", "a/b", "a/b/c", "a/b/d", "e", "e/f", "e/f/g"} {
		dirTargets = append(dirTargets, filepath.Join(dir, filepath.FromSlash(d)))
	}

	collect := func(targets []string) (*countingHost, int) {
		host := &countingHost{cwd: dir, reads: map[string]int{}}
		c := newCollector(host, dir, slog.Default())
		included := 0
		for _, target := range targets {
			files, err := c.Collect(context.Background(), target, Options{})
			if err != nil {
				t.Fatal(err)
			}
			included += len(files)
		}
		return host, included
	}

	host, included := collect(fileTargets)
	if included != len(fileTargets) {
		t.Fatalf("included %d named files, want all %d", included, len(fileTargets))
	}
	if len(host.reads) != 0 {
		t.Errorf("named files read %v, want nothing", host.reads)
	}

	host, _ = collect(dirTargets)
	total := 0
	for name, n := range host.reads {
		if n != 1 {
			t.Errorf("%s read %d times, want once", name, n)
		}
		total += n
	}
	// The root and a, a/b, a/b/c, a/b/d, e, e/f, e/f/g: three ignore files
	// and a .git each, and above the repository an .ignore, an .lxignore and a
	// .git per directory.
	if want := 8*4 + 3*strings.Count(dir, string(filepath.Separator)); total != want {
		t.Errorf("%d reads for %d walked directories, want %d", total, len(dirTargets), want)
	}
}

func paths(files []sources.InputFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// A target that does not exist is an error, which names the path stat'ed.
func TestCollect_MissingTarget(t *testing.T) {
	files, err := NewCollector(t.TempDir(), nil).Collect(context.Background(), "nope.go", Options{})
	var pe *fs.PathError
	if !errors.As(err, &pe) || pe.Op != "stat" || len(files) != 0 {
		t.Fatalf("got %v, %v; want a stat error and no inputs", paths(files), err)
	}
}

// A URL is one input, fetched only when rendered.
func TestCollect_URL(t *testing.T) {
	url := "https://example.com/repo.zip"
	files, err := NewCollector("", nil).Collect(context.Background(), url, Options{})
	if err != nil {
		t.Fatal(err)
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
	files, err := NewCollector("", nil).Collect(ctx, dir, Options{})
	if !errors.Is(err, context.Canceled) || len(files) != 0 {
		t.Fatalf("got %v, %v; want context.Canceled and no inputs", paths(files), err)
	}
}
