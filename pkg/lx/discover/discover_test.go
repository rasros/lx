package discover

import (
	"archive/zip"
	"context"
	"errors"
	"io/fs"
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
		c := &Collector{cache: walker.NewCache(host)}
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

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// The includes select archive entries the way they select walked files,
// relative to the archive root, and prune its directories the same way.
func TestCollect_ArchiveEntriesUseTheSelector(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	writeZip(t, filepath.Join(tmp, "archive.zip"), map[string]string{
		"hello.txt":       "hello\n",
		"nested/world.go": "package nested\n",
		"src/main.go":     "package main\n",
	})

	for _, tc := range []struct {
		includes []string
		want     []string
	}{
		{[]string{"*.go"}, []string{"archive.zip/nested/world.go", "archive.zip/src/main.go"}},
		{[]string{"src/*.go"}, []string{"archive.zip/src/main.go"}},
	} {
		files, err := NewCollector().Collect(context.Background(), "archive.zip",
			Options{ExpandArchives: true, Includes: tc.includes})
		if err != nil {
			t.Fatal(err)
		}
		got := paths(files)
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("includes %v: got %v, want %v", tc.includes, got, tc.want)
		}
	}
}

// A target that does not exist is an error, which names the path stat'ed.
func TestCollect_MissingTarget(t *testing.T) {
	t.Chdir(t.TempDir())
	files, err := NewCollector().Collect(context.Background(), "nope.go", Options{})
	var pe *fs.PathError
	if !errors.As(err, &pe) || pe.Op != "stat" || len(files) != 0 {
		t.Fatalf("got %v, %v; want a stat error and no inputs", paths(files), err)
	}
}

// A URL is one input, fetched only when rendered.
func TestCollect_URL(t *testing.T) {
	url := "https://example.com/repo.zip"
	files, err := NewCollector().Collect(context.Background(), url, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(files); !slices.Equal(got, []string{url}) {
		t.Fatalf("got %v, want [%s]", got, url)
	}
}
