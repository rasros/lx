package walker

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// hostWalker returns a walker reading ignore files on the OS, seen from cwd,
// that skips hidden paths.
func hostWalker(cwd string, base ...string) *Walker {
	return New(Options{Base: base, IgnoreEnabled: true, SkipHidden: true, Cache: NewCache(cwd, slog.Default())})
}

// hostFrame returns the frame of dir, seen from cwd.
func hostFrame(w *Walker, dir string) *frame { return w.cache.hostTree().frame(w, dir) }

// walkPath walks the absolute path root and returns the absolute paths of the
// files it includes.
func walkPath(t *testing.T, w *Walker, root string) []string {
	t.Helper()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := root
	if !info.IsDir() {
		dir = filepath.Dir(root)
	}
	var got []string
	err = w.WalkPath(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			got = append(got, filepath.Join(dir, filepath.FromSlash(p)))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	return got
}

// writeTree creates each file below dir, with its directories.
func writeTree(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, f := range files {
		mustMkdir(t, dir, filepath.Dir(f))
		mustWriteFile(t, dir, f, "x")
	}
}

// treeRel is dir's path from the root of the host tree, as rules match it.
func treeRel(dir string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(dir)), "/")
}

// rulePatterns lists the rules loaded at or below root, each with its base
// path relative to root. Rules from above root, such as an .ignore in a
// shared temp directory, are left out.
func rulePatterns(rules []Rule, root string) []string {
	top := treeRel(root)
	var out []string
	for _, r := range rules {
		var base string
		switch {
		case r.BasePath == top:
			base = "."
		case strings.HasPrefix(r.BasePath, top+"/"):
			base = strings.TrimPrefix(r.BasePath, top+"/")
		default:
			continue
		}
		p := r.Pattern
		if r.Negate {
			p = "!" + p
		}
		out = append(out, p+"@"+base)
	}
	return out
}

func TestFrame_RulesFromEveryAncestor(t *testing.T) {
	outer := t.TempDir()
	mustWriteFile(t, outer, ".gitignore", "outer.txt\n")
	mustWriteFile(t, outer, ".ignore", "*.ign\n")
	mustMkdir(t, outer, "repo/a/b")
	repo := filepath.Join(outer, "repo")
	mustGitDir(t, outer, "gitdirs/repo")
	mustWriteFile(t, repo, ".git", "gitdir: "+filepath.Join(outer, "gitdirs", "repo")+"\n")
	mustWriteFile(t, repo, ".gitignore", "*.log\n")
	mustWriteFile(t, repo, "a/.ignore", "tmp/\n")

	// The .gitignore above the repository is not git's, so it does not
	// apply, while the .ignore above it does. The working directory plays
	// no part.
	want := []string{"*.ign@.", "*.log@repo", "tmp/@repo/a"}
	for _, cwd := range []string{outer, filepath.Join(repo, "a", "b"), t.TempDir()} {
		w := hostWalker(cwd)
		if got := rulePatterns(hostFrame(w, filepath.Join(repo, "a", "b")).rules, outer); !reflect.DeepEqual(got, want) {
			t.Errorf("from %s: got rules %v, want %v", cwd, got, want)
		}
	}
}

func TestFrame_GitRulesNeedARepo(t *testing.T) {
	top := t.TempDir()
	if n := newRuleTree(hostTree{log: slog.Default()}, true).node(top); n.inRepo {
		t.Skip("temp dir is inside a git repository")
	}
	mustWriteFile(t, top, ".gitignore", "*.tmp\n")
	mustWriteFile(t, top, ".lxignore", "*.lx\n")
	mustMkdir(t, top, "src")

	w := New(Options{Base: []string{"*.base"}, GitExcludes: []string{"*.git"}, IgnoreEnabled: true, Cache: NewCache(filepath.Join(top, "src"), slog.Default())})
	if got := rulePatterns(hostFrame(w, filepath.Join(top, "src")).rules, top); !reflect.DeepEqual(got, []string{"*.lx@."}) {
		t.Fatalf("expected only .lxignore outside a repository, got %v", got)
	}
	if f := hostFrame(w, filepath.Join(top, "src")); f.rules[0].Pattern != "*.base" {
		t.Fatalf("expected the base rules to apply outside a repository, got %v", f.rules)
	}
	for _, r := range hostFrame(w, filepath.Join(top, "src")).rules {
		if r.Pattern == "*.git" {
			t.Fatal("expected git's global excludes not to apply outside a repository")
		}
	}
}

func TestFrame_InfoExcludeComesFirst(t *testing.T) {
	repo := t.TempDir()
	mustGitDir(t, repo, ".git")
	mustMkdir(t, repo, ".git/info")
	mustMkdir(t, repo, "sub")
	mustWriteFile(t, repo, ".git/info/exclude", "*.md\n")
	mustWriteFile(t, repo, ".gitignore", "!README.md\n")

	f := hostFrame(hostWalker(repo), filepath.Join(repo, "sub"))
	if got := rulePatterns(f.rules, repo); !reflect.DeepEqual(got, []string{"*.md@.", "!README.md@."}) {
		t.Fatalf("expected info/exclude before .gitignore, got %v", got)
	}
	if isIgnored(treeRel(repo)+"/sub/README.md", false, f.rules) {
		t.Fatal("expected .gitignore negation to override info/exclude")
	}
	if !isIgnored(treeRel(repo)+"/sub/notes.md", false, f.rules) {
		t.Fatal("expected info/exclude to ignore notes.md")
	}
}

func TestWalkPath_RepoBoundaries(t *testing.T) {
	top := t.TempDir()
	for _, d := range []string{"repo/.git/info", "repo/.git/modules/mod/info", "repo/sub/mod", "repo/sub/inner/.git/info"} {
		mustMkdir(t, top, d)
	}
	for _, d := range []string{"repo/.git", "repo/.git/modules/mod", "repo/sub/inner/.git"} {
		mustGitDir(t, top, d)
	}
	mustWriteFile(t, top, "repo/.git/info/exclude", "*.local\n")
	mustWriteFile(t, top, "repo/.gitignore", "*.tmp\n")
	mustWriteFile(t, top, "repo/a.local", "x")
	mustWriteFile(t, top, "repo/a.go", "x")

	// A submodule: its .git file points into the outer repository.
	mustWriteFile(t, top, "repo/sub/mod/.git", "gitdir: ../../.git/modules/mod\n")
	mustWriteFile(t, top, "repo/.git/modules/mod/info/exclude", "*.b\n")
	mustWriteFile(t, top, "repo/sub/mod/x.b", "x")
	mustWriteFile(t, top, "repo/sub/mod/y.local", "x")
	mustWriteFile(t, top, "repo/sub/mod/z.tmp", "x")

	// A nested repository with its own git directory.
	mustWriteFile(t, top, "repo/sub/inner/.git/info/exclude", "*.c\n")
	mustWriteFile(t, top, "repo/sub/inner/x.c", "x")
	mustWriteFile(t, top, "repo/sub/inner/y.local", "x")

	got := map[string]bool{}
	for _, p := range walkPath(t, hostWalker(top), top) {
		rel, _ := filepath.Rel(top, p)
		got[filepath.ToSlash(rel)] = true
	}
	for p, want := range map[string]bool{
		"repo/a.go":              true,
		"repo/a.local":           false, // outer info/exclude, found from above the repo
		"repo/sub/mod/x.b":       false, // the submodule's own info/exclude
		"repo/sub/mod/y.local":   true,  // outer info/exclude stops at the submodule
		"repo/sub/mod/z.tmp":     true,  // outer .gitignore stops at the submodule
		"repo/sub/inner/x.c":     false,
		"repo/sub/inner/y.local": true,
	} {
		if got[p] != want {
			t.Errorf("%s included = %v, want %v (walked %v)", p, got[p], want, got)
		}
	}

	for _, p := range walkPath(t, hostWalker(top), filepath.Join(top, "repo")) {
		if filepath.Base(p) == "a.local" {
			t.Fatal("expected a repository walked by name to honor its info/exclude")
		}
	}
}

// Overrides match a path as its root was named, as ripgrep's -g globs do:
// a relative root as written, uncleaned, an absolute one below the working
// directory with it stripped from the front.
func TestCache_Anchor(t *testing.T) {
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(top, "real", "e1")
	mustMkdir(t, top, "real/e1/sub")
	mustMkdir(t, top, "L")
	link := filepath.Join(top, "L", "e")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	other := filepath.Join(top, "other")
	slash := filepath.ToSlash
	for _, tt := range []struct{ cwd, root, rel, want string }{
		{link, link, ".", "."},
		{link, filepath.Join(link, "sub"), "x", "sub/x"},
		{link, other, "x", slash(other) + "/x"},
		{real, top, "other/x", slash(top) + "/other/x"},
		{real, "../x", "y", "../x/y"},
		{real, "./sub/", "y", "sub/y"},
		{real, "src/../x", "y", "src/../x/y"},
		{real, "sub", ".", "sub"},
	} {
		if got := NewCache(tt.cwd, slog.Default()).Anchor(tt.root).Path(tt.rel); got != tt.want {
			t.Errorf("cwd %s, root %s, path %s: got %q, want %q", tt.cwd, tt.root, tt.rel, got, tt.want)
		}
	}
}

// A shared cache reads each ignore file once: its walks keep the rules they
// first read, while a new cache reads the file as it is now. Walking a named
// file reads no ignore files.
func TestCache_KeepsTheRulesItRead(t *testing.T) {
	top := t.TempDir()
	mustGitDir(t, top, ".git")
	mustWriteFile(t, top, ".gitignore", "*.log\n")
	writeTree(t, top, "a/b/c.go", "a/b/d.log")
	goFile, logFile := filepath.Join(top, "a", "b", "c.go"), filepath.Join(top, "a", "b", "d.log")
	walk := func(cache *Cache, root string) []string {
		return walkPath(t, New(Options{IgnoreEnabled: true, SkipHidden: true, Cache: cache}), root)
	}

	named, shared := NewCache(top, slog.Default()), NewCache(top, slog.Default())
	if got := walk(named, logFile); !reflect.DeepEqual(got, []string{logFile}) {
		t.Fatalf("named file: got %v", got)
	}
	if got := walk(shared, top); !reflect.DeepEqual(got, []string{goFile}) {
		t.Fatalf("first walk: got %v", got)
	}

	mustWriteFile(t, top, ".gitignore", "*.go\n")
	if got := walk(shared, filepath.Join(top, "a")); !reflect.DeepEqual(got, []string{goFile}) {
		t.Errorf("shared cache: got %v, want the rules it first read", got)
	}
	for name, cache := range map[string]*Cache{"named": named, "new": NewCache(top, slog.Default())} {
		if got := walk(cache, filepath.Join(top, "a")); !reflect.DeepEqual(got, []string{logFile}) {
			t.Errorf("%s cache: got %v, want the rules as they are now", name, got)
		}
	}
}
