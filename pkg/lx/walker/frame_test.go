package walker

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// testHost is the OS seen from a fixed working directory. It counts the
// ignore files read and the .git entries stat'ed.
type testHost struct {
	OSHost
	cwd string

	mu    sync.Mutex
	reads map[string]int
}

func newTestHost(cwd string) *testHost { return &testHost{cwd: cwd, reads: map[string]int{}} }

func (h *testHost) Getwd() (string, error) { return h.cwd, nil }

func (h *testHost) ReadFile(name string) ([]byte, error) {
	h.count(name)
	return os.ReadFile(name)
}

func (h *testHost) Stat(name string) (fs.FileInfo, error) {
	h.count(name)
	return os.Stat(name)
}

func (h *testHost) count(name string) {
	switch filepath.Base(name) {
	case ".gitignore", ".ignore", ".lxignore", ".git":
		h.mu.Lock()
		h.reads[name]++
		h.mu.Unlock()
	}
}

func (h *testHost) total() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.reads {
		n += c
	}
	return n
}

// hostWalker returns a walker reading ignore files on the OS, seen from cwd,
// that skips hidden paths.
func hostWalker(cwd string, base ...string) *Walker {
	return New(Options{Base: base, IgnoreEnabled: true, SkipHidden: true, Cache: NewCache(newTestHost(cwd))})
}

// hostFrame returns the frame of dir, seen from cwd.
func hostFrame(w *Walker, dir string) *frame { return w.cache.hostTree().frame(w, dir) }

func walkFrom(t *testing.T, w *Walker, dir, root string) []string {
	t.Helper()
	var walked []string
	err := w.Walk(os.DirFS(dir), root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			walked = append(walked, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return walked
}

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
	_, err = w.WalkPath(root, func(p string, d fs.DirEntry, err error) error {
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

func TestFrame_Ignored(t *testing.T) {
	repo := t.TempDir()
	mustGitDir(t, repo, ".git")
	mustWriteFile(t, repo, ".gitignore", "/build\ndist\ngen/**\nout\nlib/*\n!lib/keep\nkp/\n!kp/x/k.go\n")
	for _, d := range []string{"build/x", "pkg/dist", "gen", "out/keep", "src/a", "lib/keep", "node_modules/pkg", "kp/x", "kp/y"} {
		mustMkdir(t, repo, d)
	}
	mustWriteFile(t, repo, "out/.gitignore", "!keep\n")

	tests := []struct {
		dir               string
		ignored, excluded bool
	}{
		{"build", true, true},
		{"build/x", true, true},
		{"pkg/dist", true, true},
		{"gen", true, true},
		{"out/keep", true, true},
		{"src/a", false, false},
		{"lib/keep", false, false},
		{"node_modules/pkg", true, true},
		// A negation below keeps the directory open, as in a walk.
		{"kp", true, false},
		{"kp/x", true, false},
		{"kp/y", true, true},
	}
	w := hostWalker(repo, "node_modules")
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			f := hostFrame(w, filepath.Join(repo, filepath.FromSlash(tt.dir)))
			if f.ignored != tt.ignored || f.excluded != tt.excluded {
				t.Fatalf("ignored, excluded = %v, %v, want %v, %v (%s)", f.ignored, f.excluded, tt.ignored, tt.excluded, f.reason)
			}
		})
	}
}

func TestFrame_GitRulesNeedARepo(t *testing.T) {
	top := t.TempDir()
	if n := newRuleTree(hostTree{h: OSHost{}}, true).node(top); n.inRepo {
		t.Skip("temp dir is inside a git repository")
	}
	mustWriteFile(t, top, ".gitignore", "*.tmp\n")
	mustWriteFile(t, top, ".lxignore", "*.lx\n")
	mustMkdir(t, top, "src")

	w := New(Options{Base: []string{"*.base"}, GitExcludes: []string{"*.git"}, IgnoreEnabled: true, Cache: NewCache(newTestHost(filepath.Join(top, "src")))})
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
	if ignored, _ := checkIgnore(treeRel(repo)+"/sub/README.md", false, f.rules, false, "", false); ignored {
		t.Fatal("expected .gitignore negation to override info/exclude")
	}
	if ignored, _ := checkIgnore(treeRel(repo)+"/sub/notes.md", false, f.rules, false, "", false); !ignored {
		t.Fatal("expected info/exclude to ignore notes.md")
	}
}

func TestFrame_InfoExcludeIgnoresParentDir(t *testing.T) {
	repo := t.TempDir()
	mustGitDir(t, repo, ".git")
	mustMkdir(t, repo, ".git/info")
	mustMkdir(t, repo, "scratch/x")
	mustWriteFile(t, repo, ".git/info/exclude", "/scratch/\n")

	f := hostFrame(hostWalker(repo), filepath.Join(repo, "scratch", "x"))
	if !f.excluded {
		t.Fatal("expected a directory below an info/exclude entry to be excluded")
	}
	if want := `parent directory scratch: rule "/scratch/" in ` + filepath.Join(repo, ".git", "info", "exclude"); f.reason != want {
		t.Fatalf("reason = %q, want %q", f.reason, want)
	}
}

func TestFrame_StrayGitDirIsNotARepo(t *testing.T) {
	cwd := t.TempDir()
	mustMkdir(t, cwd, "proj/.git/info")
	mustWriteFile(t, cwd, "proj/.git/info/exclude", "*.go\n")
	mustWriteFile(t, cwd, ".lxignore", "*.tmp\n")
	mustMkdir(t, cwd, "proj/sub")

	f := hostFrame(hostWalker(cwd), filepath.Join(cwd, "proj", "sub"))
	if got := rulePatterns(f.rules, cwd); !reflect.DeepEqual(got, []string{"*.tmp@."}) {
		t.Fatalf("expected only the .lxignore rule, not the stray info/exclude, got %v", got)
	}
}

// Git's rules start over at a nested repository: base rules and git's
// global excludes move to its root, followed by .ignore rules from above, its
// info/exclude and its own ignore files. The enclosing repository still
// decides whether the nested one is ignored.
func TestFrame_RepoBoundaryResets(t *testing.T) {
	top := t.TempDir()
	mustGitDir(t, top, ".git")
	mustWriteFile(t, top, ".gitignore", "*.tmp\nvendor/\n")
	mustWriteFile(t, top, ".ignore", "*.ign\n")
	mustGitDir(t, top, "lib/inner/.git")
	mustMkdir(t, top, "lib/inner/.git/info")
	mustWriteFile(t, top, "lib/inner/.git/info/exclude", "*.c\n")
	mustWriteFile(t, top, "lib/inner/.gitignore", "*.o\n")
	mustGitDir(t, top, "vendor/dep/.git")

	for _, cwd := range []string{top, filepath.Join(top, "vendor", "dep")} {
		w := New(Options{Base: []string{"*.base"}, GitExcludes: []string{"*.git"}, IgnoreEnabled: true, Cache: NewCache(newTestHost(cwd))})
		inner := hostFrame(w, filepath.Join(top, "lib", "inner"))
		want := []string{"*.base@lib/inner", "*.git@lib/inner", "*.ign@.", "*.c@lib/inner", "*.o@lib/inner"}
		if got := rulePatterns(inner.rules, top); !reflect.DeepEqual(got, want) {
			t.Fatalf("got rules %v, want %v", got, want)
		}
		if inner.ignored {
			t.Fatalf("unexpected ignored nested repository %+v", inner)
		}
		if dep := hostFrame(w, filepath.Join(top, "vendor", "dep")); !dep.excluded {
			t.Fatalf("from %s: expected a nested repository in an ignored directory to be excluded", cwd)
		}
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

	if walked := walkPath(t, hostWalker(top), filepath.Join(top, "repo", "a.local")); len(walked) != 0 {
		t.Fatalf("expected a file named from above its repository to honor info/exclude, got %v", walked)
	}
}

func TestWalkPath_IgnoredRootSkipsEverything(t *testing.T) {
	top := t.TempDir()
	mustGitDir(t, top, ".git")
	mustWriteFile(t, top, ".gitignore", "proj/\n")
	writeTree(t, top, "proj/a.go")
	proj := filepath.Join(top, "proj")

	w := hostWalker(top)
	reason, err := w.WalkPath(proj, func(string, fs.DirEntry, error) error {
		t.Fatal("expected nothing below an ignored root")
		return nil
	})
	if err != nil || reason != `parent directory proj: rule "proj/" in .gitignore` {
		t.Fatalf("got reason %q, err %v", reason, err)
	}
	if walked := walkPath(t, w, filepath.Join(proj, "a.go")); len(walked) != 0 {
		t.Fatalf("expected a file in an ignored root to be ignored, got %v", walked)
	}

	w = New(Options{Cache: NewCache(newTestHost(top))})
	if walked := walkPath(t, w, filepath.Join(proj, "a.go")); len(walked) != 1 {
		t.Fatalf("expected disabled ignores to include the file, got %v", walked)
	}
}

// Every way of naming a path agrees: walking the repository, walking a
// directory, naming each file, from the root or from a subdirectory.
func TestWalkPath_EntryPathsAgree(t *testing.T) {
	top := t.TempDir()
	mustGitDir(t, top, ".git")
	mustWriteFile(t, top, ".gitignore", "build/\n!build/keep.js\n/x/local.txt\n*.log\n")
	writeTree(t, top, "build/keep.js", "build/out.js", "build/sub/keep.js", "x/local.txt", "x/main.go", "x/run.log", "main.go")

	all := []string{"build/keep.js", "build/out.js", "build/sub/keep.js", "main.go", "x/local.txt", "x/main.go", "x/run.log"}
	want := []string{"build/keep.js", "main.go", "x/main.go"}

	for _, cwd := range []string{top, filepath.Join(top, "x"), filepath.Join(top, "build")} {
		w := hostWalker(cwd)
		var walked []string
		for _, p := range walkPath(t, w, top) {
			rel, _ := filepath.Rel(top, p)
			walked = append(walked, filepath.ToSlash(rel))
		}
		if !reflect.DeepEqual(walked, want) {
			t.Errorf("from %s: walk included %v, want %v", cwd, walked, want)
		}

		var named []string
		for _, f := range all {
			if len(walkPath(t, w, filepath.Join(top, filepath.FromSlash(f)))) == 1 {
				named = append(named, f)
			}
		}
		if !reflect.DeepEqual(named, want) {
			t.Errorf("from %s: named files included %v, want %v", cwd, named, want)
		}

		var dirs []string
		for _, d := range []string{"build", "x"} {
			for _, p := range walkPath(t, w, filepath.Join(top, d)) {
				rel, _ := filepath.Rel(top, p)
				dirs = append(dirs, filepath.ToSlash(rel))
			}
		}
		if wantDirs := []string{"build/keep.js", "x/main.go"}; !reflect.DeepEqual(dirs, wantDirs) {
			t.Errorf("from %s: directory walks included %v, want %v", cwd, dirs, wantDirs)
		}
	}
}

func TestCache_ConcurrentWalks(t *testing.T) {
	top := t.TempDir()
	mustGitDir(t, top, ".git")
	mustWriteFile(t, top, ".gitignore", "*.log\n")
	writeTree(t, top, "a/b/c.go", "a/b/d.log", "e/f.go")

	cache := NewCache(newTestHost(top))
	var wg sync.WaitGroup
	results := make([][]string, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := New(Options{IgnoreEnabled: true, SkipHidden: true, Cache: cache})
			results[i] = walkPath(t, w, top)
		}()
	}
	wg.Wait()
	for _, got := range results {
		if !reflect.DeepEqual(got, results[0]) || len(got) != 2 {
			t.Fatalf("walks disagree: %v vs %v", got, results[0])
		}
	}
}

// Naming N files in a tree of D directories reads each ignore file once with a
// shared cache, and about N times the depth without one.
func TestCache_ReadsEachIgnoreFileOnce(t *testing.T) {
	top := t.TempDir()
	mustGitDir(t, top, ".git")
	mustWriteFile(t, top, ".gitignore", "*.log\n")
	var files []string
	dirs := map[string]bool{top: true}
	for _, d := range []string{"a/b/c", "a/b/d", "a/e/f", "g/h/i"} {
		for i := range 10 {
			p := filepath.Join(top, filepath.FromSlash(d), "f"+strings.Repeat("x", i)+".go")
			mustMkdir(t, top, d)
			mustWriteFile(t, filepath.Dir(p), filepath.Base(p), "x")
			files = append(files, p)
			for dir := filepath.Dir(p); dir != top; dir = filepath.Dir(dir) {
				dirs[dir] = true
			}
		}
	}

	shared := newTestHost(top)
	cache := NewCache(shared)
	for _, f := range files {
		w := New(Options{IgnoreEnabled: true, Cache: cache})
		if got := walkPath(t, w, f); len(got) != 1 {
			t.Fatalf("expected %s to be included, got %v", f, got)
		}
	}
	for name, n := range shared.reads {
		if n != 1 {
			t.Errorf("%s read %d times, want once", name, n)
		}
	}
	// Three ignore files and a .git per directory, plus the .git of each
	// directory above the repository with its .ignore and .lxignore.
	if max := 4*len(dirs) + 3*strings.Count(top, string(filepath.Separator)); shared.total() > max {
		t.Errorf("shared cache: %d reads, want at most %d", shared.total(), max)
	}

	unshared := newTestHost(top)
	for _, f := range files {
		w := New(Options{IgnoreEnabled: true, Cache: NewCache(unshared)})
		walkPath(t, w, f)
	}
	t.Logf("%d files in %d directories: %d reads with a shared cache, %d with one cache per file",
		len(files), len(dirs), shared.total(), unshared.total())
	if unshared.total() < len(files)*4*4 {
		t.Errorf("expected about N*4*depth reads without a shared cache, got %d", unshared.total())
	}
}
