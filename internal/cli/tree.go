package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rasros/lx/pkg/lx"
)

// treePrecomp holds the results of tree pre-computation.
type treePrecomp struct {
	// strings maps each tree/tree-only op index to its rendered ASCII tree.
	strings map[int]string
	// skipFileOps contains file op indices whose content should be suppressed
	// because they belong to a tree-only group.
	skipFileOps map[int]bool
}

// precomputeTreeStrings scans ops for tree groups and pre-walks their files,
// returning precomputed tree strings and the set of file ops to suppress.
//
// A tree group is a maximal run of CmdAction ops with no CmdInterleaved ops or
// section ops between them. If a group contains a "tree" or "tree-only" op,
// all FILE/file ops in that group are walked to collect paths for the tree.
// Groups containing a "tree-only" op suppress their files from the stream.
func precomputeTreeStrings(ops []Op, cfg *lx.Config, globalIgnoreRules []string) treePrecomp {
	result := treePrecomp{
		strings:     make(map[int]string),
		skipFileOps: make(map[int]bool),
	}

	var simIncludes, simExcludes []string
	var groupFileOpIdxs []int
	var groupTreeOpIdxs []int
	groupHasTreeOnly := false

	flushGroup := func(includes, excludes []string) {
		if len(groupTreeOpIdxs) > 0 && len(groupFileOpIdxs) > 0 {
			var allPaths []string
			for _, idx := range groupFileOpIdxs {
				allPaths = append(allPaths, collectTreePaths(ops[idx], cfg, includes, excludes, globalIgnoreRules)...)
			}
			if len(allPaths) > 0 {
				treeStr := buildASCIITree(allPaths)
				for _, treeIdx := range groupTreeOpIdxs {
					result.strings[treeIdx] = treeStr
				}
			}
		}
		if groupHasTreeOnly {
			for _, fileIdx := range groupFileOpIdxs {
				result.skipFileOps[fileIdx] = true
			}
		}
		groupFileOpIdxs = nil
		groupTreeOpIdxs = nil
		groupHasTreeOnly = false
	}

	for i, op := range ops {
		if op.Type == CmdInterleaved {
			flushGroup(simIncludes, simExcludes)
			switch op.Action {
			case "include":
				simIncludes = append(simIncludes, op.Value)
			case "exclude":
				simExcludes = append(simExcludes, op.Value)
			case "reset-filters":
				simIncludes, simExcludes = nil, nil
			}
			continue
		}
		if op.Type != CmdAction {
			continue
		}
		switch op.Action {
		case "section":
			flushGroup(simIncludes, simExcludes)
		case "tree":
			groupTreeOpIdxs = append(groupTreeOpIdxs, i)
		case "tree-only":
			groupTreeOpIdxs = append(groupTreeOpIdxs, i)
			groupHasTreeOnly = true
		case "FILE", "file":
			groupFileOpIdxs = append(groupFileOpIdxs, i)
		}
	}
	flushGroup(simIncludes, simExcludes)

	return result
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
