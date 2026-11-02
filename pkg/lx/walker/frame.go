package walker

import (
	"fmt"
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

// Cache holds the ignore rules found on the host, keyed by absolute
// directory, so the walkers of one run read each ignore file and look each
// repository up once. It is safe for concurrent use.
type Cache struct {
	host Host
	once sync.Once
	ht   hostTree
	tree *ruleTree
}

// NewCache returns an empty cache over host, or the OS when host is nil. The
// host's working directory is read once, on first use.
func NewCache(host Host) *Cache {
	if host == nil {
		host = OSHost{}
	}
	return &Cache{host: host}
}

func (c *Cache) hostTree() *ruleTree {
	c.once.Do(func() {
		cwd, err := c.host.Getwd()
		if err != nil {
			slog.Debug("Failed to read the working directory", "error", err)
			cwd = ""
		} else {
			cwd = filepath.Clean(cwd)
		}
		c.ht = hostTree{h: c.host, cwd: cwd}
		c.tree = newRuleTree(c.ht, true)
	})
	return c.tree
}

// tree is a namespace of directories that frames are built over: absolute
// paths on the host, or paths within a walked fs.FS.
type tree interface {
	parent(dir string) (string, bool)
	child(dir, name string) string
	// onCwdPath reports whether dir is the working directory or above it.
	onCwdPath(dir string) bool
	gitDir(dir string) (g gitFS, gitDir string, ok bool)
	readFile(dir, name string) (data []byte, source string, err error)
	// display names a path in an ignore reason.
	display(p string) string
}

// hostTree is the host filesystem, seen from the working directory cwd.
type hostTree struct {
	h   Host
	cwd string
}

func (t hostTree) parent(dir string) (string, bool) {
	p := filepath.Dir(dir)
	return p, p != dir
}

func (hostTree) child(dir, name string) string { return filepath.Join(dir, name) }

func (t hostTree) onCwdPath(dir string) bool {
	if t.cwd == "" || !strings.HasPrefix(t.cwd, dir) {
		return false
	}
	return len(t.cwd) == len(dir) || strings.HasSuffix(dir, string(filepath.Separator)) || t.cwd[len(dir)] == filepath.Separator
}

func (t hostTree) gitDir(dir string) (gitFS, string, bool) {
	g := hostGitFS{t.h}
	gitDir, ok := findGitDir(g, dir)
	return g, gitDir, ok
}

func (t hostTree) readFile(dir, name string) ([]byte, string, error) {
	p := filepath.Join(dir, name)
	data, err := t.h.ReadFile(p)
	logReadError(p, err)
	return data, t.display(p), err
}

// display names p relative to the working directory when it is below it,
// and by its absolute path otherwise.
func (t hostTree) display(p string) string {
	if rel, ok := t.below(p); ok {
		return rel
	}
	return p
}

// below returns p relative to the working directory, reporting false when p
// is not at or below it.
func (t hostTree) below(p string) (string, bool) {
	if t.cwd == "" {
		return "", false
	}
	rel, err := filepath.Rel(t.cwd, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// fsTree is a walked fs.FS that stands alone: its root is the top, and a
// .git pointing outside it cannot be followed.
type fsTree struct{ fsys fs.FS }

func (fsTree) parent(dir string) (string, bool) {
	if dir == "." {
		return "", false
	}
	return path.Dir(dir), true
}

func (fsTree) child(dir, name string) string { return joinRel(dir, name) }
func (fsTree) onCwdPath(dir string) bool     { return dir == "." }
func (fsTree) display(p string) string       { return p }

func (t fsTree) gitDir(dir string) (gitFS, string, bool) {
	g := subGitFS{t.fsys}
	gitDir, ok := findGitDir(g, dir)
	return g, gitDir, ok
}

func (t fsTree) readFile(dir, name string) ([]byte, string, error) {
	p := joinRel(dir, name)
	data, err := fs.ReadFile(t.fsys, p)
	logReadError(p, err)
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
	// parent is nil for a top directory, which nothing above affects: the
	// root of the tree, and, on the path to the working directory, a
	// repository root or a directory outside any repository. Below the
	// working directory, or beside it, the rules of the directories above
	// keep applying, so every way of naming a path agrees.
	parent *node
	top    bool
	repo   bool // a repository root: its rules start over
	inRepo bool
	g      gitFS
	gitDir string
	// rel is the directory's path relative to its anchor, the top or
	// repository root its rules start from, and relInParent the same
	// relative to its parent's anchor, which differ at a repository root.
	rel, relInParent string

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
	parentDir, ok := rt.t.parent(dir)
	switch {
	case !ok:
		n.top = true
	case rt.t.onCwdPath(dir) && n.repo:
		n.top = true
	default:
		p := rt.node(parentDir)
		if rt.t.onCwdPath(dir) && !p.inRepo {
			n.top = true
			break
		}
		n.parent = p
		n.relInParent = joinRel(p.rel, path.Base(filepath.ToSlash(dir)))
		if !n.repo {
			n.rel = n.relInParent
		}
	}
	n.inRepo = n.repo || (n.parent != nil && n.parent.inRepo)
	rt.nodes[dir] = n
	return n
}

// ownRules returns the rules dir itself adds. Git ranks info/exclude below
// every ignore file, so it comes first.
func (rt *ruleTree) ownRules(n *node) []Rule {
	if n.loaded || !rt.ignoreFiles {
		return n.own
	}
	n.loaded = true
	if n.repo {
		n.own = infoExclude(n.g, n.gitDir, ".")
	}
	for _, name := range []string{".gitignore", ".ignore", ".lxignore"} {
		data, source, err := rt.t.readFile(n.dir, name)
		if err == nil {
			n.own = append(n.own, parseRuleFile(data, n.rel, source)...)
		}
	}
	return n.own
}

// frameSet holds the frames built for one set of base rules.
type frameSet struct {
	base   []Rule
	frames map[string]*frame
}

// frame is the ignore state of a directory: the rules in effect in it,
// relative to its node's anchor, and whether it is ignored itself.
type frame struct {
	node  *node
	rules []Rule
	// ignored reports that the directory is ignored. Unless it is also
	// excluded, a negation could re-include something below it, so a walk
	// still enters it, carrying the ignored state down. excluded reports
	// that nothing below it is included.
	ignored, excluded bool
	reason            string
}

// frame returns the frame of dir for w's base rules.
func (rt *ruleTree) frame(w *Walker, dir string) *frame {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	set, ok := rt.sets[w.baseKey]
	if !ok {
		set = &frameSet{base: w.base, frames: map[string]*frame{}}
		rt.sets[w.baseKey] = set
	}
	return rt.frameOf(set, dir)
}

// isTop reports whether dir is a top directory, which starts over.
func (rt *ruleTree) isTop(dir string) bool {
	if !rt.t.onCwdPath(dir) {
		if _, ok := rt.t.parent(dir); ok {
			return false
		}
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.node(dir).top
}

func (rt *ruleTree) frameOf(set *frameSet, dir string) *frame {
	if f, ok := set.frames[dir]; ok {
		return f
	}
	n := rt.node(dir)
	f := &frame{node: n}
	var parentRules []Rule
	if n.parent != nil {
		pf := rt.frameOf(set, n.parent.dir)
		parentRules = pf.rules
		if pf.excluded {
			f.ignored, f.excluded, f.reason = true, true, pf.reason
		} else {
			ignored, descend, reason, own := judgeDir(pf.rules, n.relInParent, false, nil, "", pf.ignored, pf.reason)
			if ignored {
				f.ignored, f.excluded, f.reason = true, !descend, dirReason(rt.t.display(dir), reason, own)
			}
		}
	}
	// The one place the rules start over: at a top directory, or at a
	// repository root, as git never applies one repository's ignore rules
	// inside another, submodules included.
	own := rt.ownRules(n)
	if n.parent == nil || n.repo {
		f.rules = make([]Rule, 0, len(set.base)+len(own))
		f.rules = append(append(f.rules, set.base...), own...)
	} else {
		f.rules = append(parentRules[:len(parentRules):len(parentRules)], own...)
	}
	set.frames[dir] = f
	return f
}

// judgeDir decides a directory: rules are those in effect in its parent,
// matched at rel, and override those of the walk, matched at walkRel.
// ignored and reason are the state carried from the parent, which a top
// directory drops. It reports whether the directory is ignored, whether a
// walk still descends into it because a negation could re-include something
// below, and the reason, own when a rule matched the directory itself.
func judgeDir(rules []Rule, rel string, top bool, override []Rule, walkRel string, ignored bool, reason string) (bool, bool, string, bool) {
	if top {
		ignored, reason = false, ""
	}
	carried := reason
	if !top {
		ignored, reason = checkIgnore(rel, true, rules, ignored, reason, true)
	}
	ignored, reason = checkIgnore(walkRel, true, override, ignored, reason, true)
	if !ignored {
		return false, false, "", false
	}
	descend := (!top && hasNestedException(rel, rules)) || (len(override) > 0 && hasNestedException(walkRel, override))
	return true, descend, reason, reason != carried
}

// dirReason is the reason given for a path below the ignored directory
// display: the rule that matched the directory, or the reason carried
// from above it.
func dirReason(display, reason string, own bool) string {
	if !own {
		return reason
	}
	return fmt.Sprintf("parent directory %s: %s", display, reason)
}
