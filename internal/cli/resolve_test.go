package cli

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

// countingHost is the OS seen from a fixed working directory, counting the
// ignore files read and the .git entries stat'ed.
type countingHost struct {
	lx.OSHost
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
// files at all. Named directories share one rule cache, so each directory's
// ignore files are read once however many walks reach it.
func TestResolveSections_IgnoreFileReads(t *testing.T) {
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

	var fileOps, dirOps []Op
	for _, d := range []string{"a/b/c", "a/b/d", "e/f/g"} {
		for i := range 20 {
			rel := d + "/f" + strings.Repeat("x", i) + ".go"
			write(rel, "package x")
			fileOps = append(fileOps, Op{Action: "FILE", Value: filepath.Join(dir, filepath.FromSlash(rel)), Type: CmdAction})
		}
		write(d+"/skip.log", "ignored in a walk")
	}
	fileOps = append(fileOps, Op{Action: "FILE", Value: filepath.Join(dir, "a", "b", "c", "skip.log"), Type: CmdAction})
	for _, d := range []string{".", "a", "a/b", "a/b/c", "a/b/d", "e", "e/f", "e/f/g"} {
		dirOps = append(dirOps, Op{Action: "FILE", Value: filepath.Join(dir, filepath.FromSlash(d)), Type: CmdAction})
	}

	resolve := func(ops []Op) (*countingHost, int) {
		host := &countingHost{cwd: dir, reads: map[string]int{}}
		rc := testResolveContext(Section{})
		rc.rules = lx.NewRuleCache(host)
		sections := []Section{{Ops: ops}}
		if unresolved := resolveSections(context.Background(), sections, rc); unresolved != 0 {
			t.Fatalf("unresolved = %d", unresolved)
		}
		included := 0
		for _, files := range sections[0].resolved {
			included += len(files)
		}
		return host, included
	}

	host, included := resolve(fileOps)
	if included != len(fileOps) {
		t.Fatalf("included %d named files, want all %d", included, len(fileOps))
	}
	if len(host.reads) != 0 {
		t.Errorf("named files read %v, want nothing", host.reads)
	}

	host, _ = resolve(dirOps)
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
		t.Errorf("%d reads for %d walked directories, want %d", total, len(dirOps), want)
	}
}
