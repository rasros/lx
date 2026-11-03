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
// directory, so the walkers of one run read each ignore file once. It is
// safe for concurrent use.
type Cache struct {
	host Host
	log  *slog.Logger
	once sync.Once
	wd   string // the working directory as the host names it
	ht   hostTree
	tree *ruleTree
}

// NewCache returns an empty cache over host, or the OS when host is nil. A
// nil log means slog.Default().
func NewCache(host Host, log *slog.Logger) *Cache {
	if host == nil {
		host = OSHost{}
	}
	return &Cache{host: host, log: orDefault(log)}
}

// orDefault returns log, or slog.Default() when log is nil.
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
// against them: as the root was named, with the working directory stripped
// from an absolute path.
type Anchor struct {
	// prefix is the root's own path and a slash, or empty at the working
	// directory.
	prefix string
}

// Anchor returns the anchor of root. As in ripgrep, root keeps its path as
// written but for a leading "./" and trailing slashes, so the paths below
// "src/../x" start with "src/../x/".
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
	if c.wd != "" {
		if rel, ok := strings.CutPrefix(p, strings.TrimSuffix(filepath.ToSlash(c.wd), "/")+"/"); ok {
			return Anchor{prefix: rel}
		}
	}
	return Anchor{prefix: p}
}

// Path returns the path to match for rel, a slash-separated path below the
// root, or "." for the root itself.
func (a Anchor) Path(rel string) string {
	if rel != "." {
		return a.prefix + rel
	}
	switch a.prefix {
	case "":
		return "."
	case "/":
		return "/"
	}
	return strings.TrimSuffix(a.prefix, "/")
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
	// gitDir reports whether dir is a repository root, and its git
	// directory, or "" when it has none to read.
	gitDir(dir string) (gitDir string, ok bool)
	readFile(dir, name string) (data []byte, source string, err error)
	// display names a path in an ignore reason.
	display(p string) string
}

// hostTree is the host filesystem seen from cwd. Its directories are real
// paths, as ripgrep resolves them.
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

func (t hostTree) gitDir(dir string) (string, bool) { return findGitDir(t.h, dir) }

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

// fsTree is a walked fs.FS that stands alone: its root is the top, and
// repositories inside it are not looked for.
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

func (t fsTree) gitDir(dir string) (string, bool) { return "", t.repoRoot && dir == "." }

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

// node is what a directory contributes to the rules: where it stands
// relative to repositories and its own ignore files.
type node struct {
	dir string
	// parent is nil only at the root of the tree. As in ripgrep, the rules
	// of every directory above a path apply to it.
	parent *node
	repo   bool // a repository root: its rules start over
	inRepo bool // in a repository, where git's own ignore rules apply
	gitDir string
	// rel is the directory's path from the root of the tree.
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
		n.gitDir, n.repo = rt.t.gitDir(dir)
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
// every ignore file, so it comes first. As in ripgrep, git's rules apply
// only inside a repository, while .ignore and .lxignore apply everywhere.
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
		n.own = withScope(rt.infoExclude(n.gitDir, n.rel), scopeGit)
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

// infoExclude reads the info/exclude rules shared by the worktrees of
// gitDir, in the directory its commondir file names, scoped to basePath.
func (rt *ruleTree) infoExclude(gitDir, basePath string) []Rule {
	if gitDir == "" {
		return nil
	}
	common := gitDir
	if data, _, err := rt.t.readFile(gitDir, "commondir"); err == nil {
		if c := strings.TrimSpace(string(data)); c != "" {
			common = resolveRelative(gitDir, c)
		}
	}
	data, _, err := rt.t.readFile(common, filepath.Join("info", "exclude"))
	if err != nil {
		return nil
	}
	return parseRuleFile(data, basePath, filepath.Join(common, "info", "exclude"))
}

// frameSet holds the frames built for one set of base rules and git global
// excludes.
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
		// Git rules start over at a repository root, submodules included.
		// The base rules move to the new root, while .ignore and .lxignore
		// rules from above keep applying, as in ripgrep.
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

// rebase returns a copy of rules matched relative to rel instead of the
// tree's root.
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
