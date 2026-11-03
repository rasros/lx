package walker

import (
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// Host is the host filesystem and working directory as WalkPath sees them.
type Host interface {
	Getwd() (string, error)
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	ReadLink(name string) (string, error)
	ReadFile(name string) ([]byte, error)
	EvalSymlinks(path string) (string, error)
	DirFS(dir string) fs.FS
}

// OSHost is the Host of the running process.
type OSHost struct{}

func (OSHost) Getwd() (string, error)                 { return os.Getwd() }
func (OSHost) Stat(name string) (fs.FileInfo, error)  { return os.Stat(name) }
func (OSHost) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }
func (OSHost) ReadLink(name string) (string, error)   { return os.Readlink(name) }
func (OSHost) ReadFile(name string) ([]byte, error)   { return os.ReadFile(name) }
func (OSHost) DirFS(dir string) fs.FS                 { return os.DirFS(dir) }
func (OSHost) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// Cache holds the ignore rules found on the host, keyed by absolute
// directory, so the walkers of one run read each ignore file and look each
// repository up once. It is safe for concurrent use. The working directory
// only shapes how paths are named in reasons, never which rules apply.
type Cache struct {
	host Host
	log  *slog.Logger
	once sync.Once
	wd   string // the working directory as the host names it
	ht   hostTree
	tree *ruleTree
}

// NewCache returns an empty cache over host, or the OS when host is nil, that
// logs to log, or slog.Default() when log is nil. The host's working
// directory is read once, on first use.
func NewCache(host Host, log *slog.Logger) *Cache {
	if host == nil {
		host = OSHost{}
	}
	return &Cache{host: host, log: orDefault(log)}
}

// orDefault returns log, or slog.Default() when log is nil. The exported
// constructors default the logger once; everything below them is given one.
func orDefault(log *slog.Logger) *slog.Logger {
	if log == nil {
		return slog.Default()
	}
	return log
}

func (c *Cache) hostTree() *ruleTree {
	c.once.Do(func() {
		cwd, err := c.host.Getwd()
		if err != nil {
			c.log.Debug("Failed to read the working directory", "error", err)
			cwd = ""
		} else {
			cwd = filepath.Clean(cwd)
		}
		c.wd = cwd
		c.ht = newHostTree(c.host, "", c.log)
		if cwd != "" {
			c.ht.cwd = c.ht.real(cwd)
		}
		c.tree = newRuleTree(c.ht, true)
	})
	return c.tree
}

// Anchor names the paths below a root as ripgrep matches its -g globs
// against them: as the root was named, relative or absolute, with the
// working directory stripped from the front of an absolute path below it.
type Anchor struct {
	// prefix is the root's own path and a slash, or empty at the working
	// directory.
	prefix string
	// strip holds the names of the working directory, each with a slash,
	// that are stripped from the paths below an absolute root above it.
	strip []string
}

// Anchor returns the anchor of root, a path named from the working
// directory or absolutely. As in ripgrep, the root keeps its path as
// written, without cleaning but for a leading "./" and trailing slashes, so
// the paths below "../x" start with "../x/" and those below "src/../x" with
// "src/../x/". The working directory may be named through a symlink and
// root by its real path, or the other way round, so either name of it is
// stripped.
func (c *Cache) Anchor(root string) Anchor {
	c.hostTree()
	p := filepath.ToSlash(root)
	for strings.HasPrefix(p, "./") {
		p = p[2:]
	}
	p = strings.TrimRight(p, "/")
	if !filepath.IsAbs(root) {
		if p == "." || p == "" {
			return Anchor{}
		}
		return Anchor{prefix: p + "/"}
	}
	p += "/"
	a := Anchor{prefix: p}
	for _, wd := range []string{c.wd, c.ht.cwd} {
		if wd == "" {
			continue
		}
		wd = strings.TrimSuffix(filepath.ToSlash(wd), "/") + "/"
		switch {
		case strings.HasPrefix(p, wd):
			return Anchor{prefix: p[len(wd):]}
		case strings.HasPrefix(wd, p):
			a.strip = append(a.strip, wd)
		}
	}
	return a
}

// Path returns the path to match for rel, a slash-separated path below the
// root, or "." for the root itself.
func (a Anchor) Path(rel string) string {
	if rel == "." {
		switch a.prefix {
		case "":
			return "."
		case "/":
			return "/"
		}
		return strings.TrimSuffix(a.prefix, "/")
	}
	p := a.prefix + rel
	for _, wd := range a.strip {
		if strings.HasPrefix(p, wd) {
			return p[len(wd):]
		}
		if len(p)+1 == len(wd) && strings.HasPrefix(wd, p) {
			return "."
		}
	}
	return p
}

// leadsToWD reports whether p, an absolute path from Path, is a directory
// above the working directory, below which paths are matched relative to it.
func (a Anchor) leadsToWD(p string) bool {
	for _, wd := range a.strip {
		if strings.HasPrefix(wd, p+"/") {
			return true
		}
	}
	return false
}

// isOutside reports whether rel, a path from filepath.Rel, leaves the
// directory it is relative to.
func isOutside(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// tree is a namespace of directories that frames are built over: absolute
// paths on the host, or paths within a walked fs.FS.
type tree interface {
	parent(dir string) (string, bool)
	child(dir, name string) string
	gitDir(dir string) (g gitFS, gitDir string, ok bool)
	readFile(dir, name string) (data []byte, source string, err error)
	// display names a path in an ignore reason.
	display(p string) string
}

// hostTree is the host filesystem, seen from the working directory cwd, with
// symlinks resolved. Directories in the tree are real paths, as ripgrep
// resolves the paths it searches.
type hostTree struct {
	h   Host
	cwd string
	log *slog.Logger
}

func newHostTree(h Host, cwd string, log *slog.Logger) hostTree {
	return hostTree{h: h, cwd: cwd, log: log}
}

// real returns p with symlinks resolved, or p itself when it cannot be.
func (t hostTree) real(p string) string {
	if r, err := t.h.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func (t hostTree) parent(dir string) (string, bool) {
	p := filepath.Dir(dir)
	return p, p != dir
}

func (hostTree) child(dir, name string) string { return filepath.Join(dir, name) }

func (t hostTree) gitDir(dir string) (gitFS, string, bool) {
	g := hostGitFS{t.h, t.log}
	gitDir, ok := findGitDir(g, dir)
	return g, gitDir, ok
}

func (t hostTree) readFile(dir, name string) ([]byte, string, error) {
	p := filepath.Join(dir, name)
	data, err := t.h.ReadFile(p)
	logReadError(t.log, p, err)
	return data, t.display(p), err
}

// display names p relative to the working directory when it is below it,
// and by its absolute path otherwise.
func (t hostTree) display(p string) string {
	if t.cwd == "" {
		return p
	}
	rel, err := filepath.Rel(t.cwd, p)
	if err != nil || isOutside(rel) {
		return p
	}
	return rel
}

// fsTree is a walked fs.FS that stands alone: its root is the top, and a
// .git pointing outside it cannot be followed.
type fsTree struct {
	fsys fs.FS
	log  *slog.Logger
	// repoRoot makes the root a repository root, whose .git the fs.FS
	// does not hold.
	repoRoot bool
}

func newFSTree(fsys fs.FS, log *slog.Logger) fsTree { return fsTree{fsys: fsys, log: log} }

func (fsTree) parent(dir string) (string, bool) {
	if dir == "." {
		return "", false
	}
	return path.Dir(dir), true
}

func (fsTree) child(dir, name string) string { return joinRel(dir, name) }
func (fsTree) display(p string) string       { return p }

func (t fsTree) gitDir(dir string) (gitFS, string, bool) {
	g := subGitFS{t.fsys, t.log}
	if t.repoRoot && dir == "." {
		return g, "", true
	}
	gitDir, ok := findGitDir(g, dir)
	return g, gitDir, ok
}

func (t fsTree) readFile(dir, name string) ([]byte, string, error) {
	p := joinRel(dir, name)
	data, err := fs.ReadFile(t.fsys, p)
	logReadError(t.log, p, err)
	return data, p, err
}

// joinRel joins a slash-separated relative path, where "." is the root.
func joinRel(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return dir + "/" + name
}

// ruleTree builds and keeps the nodes and frames of one tree.
type ruleTree struct {
	t           tree
	ignoreFiles bool

	mu    sync.Mutex
	nodes map[string]*node
	sets  map[string]*frameSet
}

func newRuleTree(t tree, ignoreFiles bool) *ruleTree {
	return &ruleTree{t: t, ignoreFiles: ignoreFiles, nodes: map[string]*node{}, sets: map[string]*frameSet{}}
}

// node is what a directory contributes to the rules, whatever the base rules:
// where it stands relative to repositories and its own ignore files.
type node struct {
	dir string
	// parent is nil only at the root of the tree. Like ripgrep, the rules of
	// every directory above a path apply to it, wherever the walk starts.
	parent *node
	repo   bool // a repository root: its rules start over
	inRepo bool // in a repository, where git's own ignore rules apply
	g      gitFS
	gitDir string
	// rel is the directory's path from the root of the tree, which every
	// rule is matched against.
	rel string

	loaded bool
	own    []Rule // info/exclude, when a repository root, then its ignore files
}

func (rt *ruleTree) node(dir string) *node {
	if n, ok := rt.nodes[dir]; ok {
		return n
	}
	n := &node{dir: dir, rel: "."}
	if rt.ignoreFiles {
		n.g, n.gitDir, n.repo = rt.t.gitDir(dir)
	}
	if parentDir, ok := rt.t.parent(dir); ok {
		p := rt.node(parentDir)
		n.parent = p
		n.rel = joinRel(p.rel, path.Base(filepath.ToSlash(dir)))
	}
	n.inRepo = n.repo || (n.parent != nil && n.parent.inRepo)
	rt.nodes[dir] = n
	return n
}

// ownRules returns the rules dir itself adds. Git ranks info/exclude below
// every ignore file, so it comes first. As in ripgrep, git's own rules,
// info/exclude and .gitignore, apply only inside a repository, while .ignore
// and .lxignore apply everywhere.
func (rt *ruleTree) ownRules(n *node) []Rule {
	if n.loaded || !rt.ignoreFiles {
		return n.own
	}
	n.loaded = true
	names := []string{".ignore", ".lxignore"}
	if n.inRepo {
		names = append([]string{".gitignore"}, names...)
	}
	if n.repo {
		n.own = withScope(infoExclude(n.g, n.gitDir, n.rel), scopeGit)
	}
	for _, name := range names {
		data, source, err := rt.t.readFile(n.dir, name)
		if err == nil {
			scope := scopeIgnore
			if name == ".gitignore" {
				scope = scopeGit
			}
			n.own = append(n.own, withScope(parseRuleFile(data, n.rel, source), scope)...)
		}
	}
	return n.own
}

// frameSet holds the frames built for one set of base rules: base, which
// apply everywhere, and git, git's global excludes, which apply only inside a
// repository.
type frameSet struct {
	base, git []Rule
	frames    map[string]*frame
}

// frame is the ignore state of a directory: the rules in effect in it.
type frame struct {
	node  *node
	rules []Rule
}

// frame returns the frame of dir for w's base rules.
func (rt *ruleTree) frame(w *Walker, dir string) *frame {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	set, ok := rt.sets[w.baseKey]
	if !ok {
		set = &frameSet{base: w.base, git: w.gitExcludes, frames: map[string]*frame{}}
		rt.sets[w.baseKey] = set
	}
	return rt.frameOf(set, dir)
}

func (rt *ruleTree) frameOf(set *frameSet, dir string) *frame {
	if f, ok := set.frames[dir]; ok {
		return f
	}
	n := rt.node(dir)
	f := &frame{node: n}
	var parentRules []Rule
	if n.parent != nil {
		parentRules = rt.frameOf(set, n.parent.dir).rules
	}
	own := rt.ownRules(n)
	switch {
	case n.parent == nil:
		f.rules = rebase(set.base, n.rel)
		if n.repo {
			f.rules = append(f.rules, rebase(set.git, n.rel)...)
		}
		f.rules = append(f.rules, own...)
	case n.repo:
		// The one place the rules start over: at a repository root, git
		// never applies one repository's rules inside another, submodules
		// included. The base rules move to the new root, while .ignore and
		// .lxignore rules from above keep applying, as in ripgrep.
		f.rules = append(rebase(set.base, n.rel), rebase(set.git, n.rel)...)
		for _, r := range parentRules {
			if r.scope == scopeIgnore {
				f.rules = append(f.rules, r)
			}
		}
		f.rules = append(f.rules, own...)
	default:
		f.rules = append(parentRules[:len(parentRules):len(parentRules)], own...)
	}
	set.frames[dir] = f
	return f
}

// ruleScope says where a rule comes from, which decides where it applies.
type ruleScope uint8

const (
	scopeBase   ruleScope = iota // base and override patterns
	scopeGit                     // git's: .gitignore, info/exclude, global excludes
	scopeIgnore                  // .ignore and .lxignore, which apply everywhere
)

func withScope(rules []Rule, scope ruleScope) []Rule {
	for i := range rules {
		rules[i].scope = scope
	}
	return rules
}

// rebase returns a copy of rules, which were parsed relative to the root of
// a tree, matched relative to its directory at rel instead.
func rebase(rules []Rule, rel string) []Rule {
	out := make([]Rule, len(rules))
	for i, r := range rules {
		r.BasePath, r.BasePathPrefix = rel, ""
		if rel != "." {
			r.BasePathPrefix = rel + "/"
		}
		out[i] = r
	}
	return out
}
