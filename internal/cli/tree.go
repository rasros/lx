package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rasros/lx/pkg/lx"
)

// precomputeTrees walks each section that contains a tree or tree-only op,
// collecting file paths and building ASCII tree strings. Within each group, section
// ops act as sub-boundaries: each section-delimited run of ops gets its own tree.
// Results are stored in the group's treeStrings and skipFileOps maps, keyed by the
// op's index within the group's Ops slice.
func precomputeTrees(groups []Section, cfg *lx.Config, globalIgnoreRules []string) {
	for i := range groups {
		computeSectionTrees(&groups[i], cfg, globalIgnoreRules)
	}
}

func computeSectionTrees(g *Section, cfg *lx.Config, globalIgnoreRules []string) {
	var subFileIdxs []int
	var subTreeIdxs []int
	subTreeOnly := false

	flush := func() {
		if len(subTreeIdxs) > 0 && len(subFileIdxs) > 0 {
			var paths []string
			for _, idx := range subFileIdxs {
				paths = append(paths, collectTreePaths(g.Ops[idx], cfg, g.Includes, g.Excludes, globalIgnoreRules)...)
			}
			if len(paths) > 0 {
				ts := buildASCIITree(paths)
				if g.treeStrings == nil {
					g.treeStrings = make(map[int]string)
				}
				for _, ti := range subTreeIdxs {
					g.treeStrings[ti] = ts
				}
			}
		}
		if subTreeOnly {
			if g.skipFileOps == nil {
				g.skipFileOps = make(map[int]bool)
			}
			for _, fi := range subFileIdxs {
				g.skipFileOps[fi] = true
			}
		}
		subFileIdxs = nil
		subTreeIdxs = nil
		subTreeOnly = false
	}

	for oi, op := range g.Ops {
		switch op.Action {
		case "section":
			flush()
		case "tree":
			subTreeIdxs = append(subTreeIdxs, oi)
		case "tree-only":
			subTreeIdxs = append(subTreeIdxs, oi)
			subTreeOnly = true
		case "FILE", "file":
			subFileIdxs = append(subFileIdxs, oi)
		}
	}
	flush()
}

// collectTreePaths walks a single FILE or file op and returns all file paths
// that would be included, using the same logic as the main processing loop.
func collectTreePaths(op Op, cfg *lx.Config, includes, excludes []string, globalIgnoreRules []string) []string {
	var paths []string

	rawPath := op.Value
	isForced := op.Action == "file"

	// Skip stdin and URLs — they can't be walked for tree purposes
	if rawPath == "-" || lx.IsHTTPURL(rawPath) {
		return paths
	}

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return paths
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return paths
	}

	if !stat.IsDir() {
		if !isForced && !lx.IsKept(rawPath, includes, excludes) {
			return paths
		}
		return append(paths, filepath.Clean(rawPath))
	}

	fsys := os.DirFS(absPath)
	displayPrefix := filepath.Clean(rawPath)

	var baseRules, overrideRules []string
	if cfg.IgnoreEnabled {
		baseRules = append(baseRules, globalIgnoreRules...)
	}
	if cfg.IgnoreHidden && !isForced {
		overrideRules = append(overrideRules, ".*")
	}
	if !isForced {
		overrideRules = append(overrideRules, excludes...)
	}

	w := lx.NewWalker(baseRules, overrideRules)
	w.IgnoreEnabled = cfg.IgnoreEnabled

	_ = w.Walk(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		var effectivePath string
		if path == "." {
			effectivePath = displayPrefix
		} else if displayPrefix == "." {
			effectivePath = filepath.FromSlash(path)
		} else {
			effectivePath = filepath.Join(displayPrefix, filepath.FromSlash(path))
		}

		if !isForced && len(includes) > 0 {
			matched := false
			for _, inc := range includes {
				if lx.IsMatch(inc, path) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		paths = append(paths, effectivePath)
		return nil
	})

	return paths
}

// treeNode is an internal node in the path trie used to build the ASCII tree.
type treeNode struct {
	children   map[string]*treeNode
	childOrder []string
}

func newTreeNode() *treeNode {
	return &treeNode{children: make(map[string]*treeNode)}
}

// buildASCIITree converts a flat list of file paths into an ASCII directory tree.
func buildASCIITree(paths []string) string {
	root := newTreeNode()

	for _, p := range paths {
		p = filepath.ToSlash(filepath.Clean(p))
		p = strings.TrimPrefix(p, "./")
		if p == "" || p == "." {
			continue
		}
		node := root
		for _, part := range strings.Split(p, "/") {
			if part == "" {
				continue
			}
			if _, ok := node.children[part]; !ok {
				node.children[part] = newTreeNode()
				node.childOrder = append(node.childOrder, part)
			}
			node = node.children[part]
		}
	}

	sortTreeNode(root)

	var buf strings.Builder
	topLevel := root.childOrder

	switch len(topLevel) {
	case 0:
		// nothing to render
	case 1:
		// Single root entry: print it as the root line, then its children
		name := topLevel[0]
		child := root.children[name]
		displayName := name
		if len(child.children) > 0 {
			displayName += "/"
		}
		buf.WriteString(displayName + "\n")
		renderTreeChildren(&buf, child, "")
	default:
		// Multiple root entries: use connector lines for all top-level entries
		renderTreeChildren(&buf, root, "")
	}

	return strings.TrimSuffix(buf.String(), "\n")
}

func sortTreeNode(node *treeNode) {
	sort.Slice(node.childOrder, func(i, j int) bool {
		ni, nj := node.childOrder[i], node.childOrder[j]
		iIsDir := len(node.children[ni].children) > 0
		jIsDir := len(node.children[nj].children) > 0
		if iIsDir != jIsDir {
			return iIsDir // directories before files
		}
		return ni < nj
	})
	for _, name := range node.childOrder {
		sortTreeNode(node.children[name])
	}
}

func renderTreeChildren(buf *strings.Builder, node *treeNode, prefix string) {
	for i, name := range node.childOrder {
		child := node.children[name]
		isLast := i == len(node.childOrder)-1

		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		displayName := name
		if len(child.children) > 0 {
			displayName += "/"
		}

		buf.WriteString(prefix + connector + displayName + "\n")
		renderTreeChildren(buf, child, childPrefix)
	}
}
