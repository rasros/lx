package cli

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rasros/lx/pkg/lx"
)

func precomputeTrees(ctx context.Context, groups []Section, globalIgnoreRules []string) {
	for i := range groups {
		computeSectionTrees(ctx, &groups[i], globalIgnoreRules)
	}
}

func computeSectionTrees(ctx context.Context, g *Section, globalIgnoreRules []string) {
	var subFileIdxs []int
	var subTreeIdxs []int
	subTreeOnly := false

	flush := func() {
		if len(subTreeIdxs) > 0 && len(subFileIdxs) > 0 {
			var paths []string
			for _, idx := range subFileIdxs {
				paths = append(paths, collectTreePaths(ctx, g.Ops[idx], g.RunCfg, g.Includes, g.Excludes, globalIgnoreRules)...)
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
			// Flush any preceding file ops so they are rendered as content,
			// not silently dropped by tree-only mode.
			flush()
			subTreeIdxs = append(subTreeIdxs, oi)
			subTreeOnly = true
		case "FILE", "file":
			subFileIdxs = append(subFileIdxs, oi)
		}
	}
	flush()
}

func collectTreePaths(ctx context.Context, op Op, runCfg lx.RunnerConfig, includes, excludes []string, globalIgnoreRules []string) []string {
	var paths []string

	rawPath := op.Value
	isForced := op.Action == "file"

	if rawPath == "-" {
		return paths
	}

	if lx.IsHTTPURL(rawPath) {
		if runCfg.ExpandArchives && lx.IsHTTPArchiveURL(rawPath) {
			return collectURLArchiveTreePaths(ctx, rawPath, runCfg.ShowHidden, includes)
		}
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
	if !runCfg.NoIgnore {
		baseRules = append(baseRules, globalIgnoreRules...)
	}
	if !runCfg.ShowHidden && !isForced {
		overrideRules = append(overrideRules, ".*")
	}
	if !isForced {
		overrideRules = append(overrideRules, excludes...)
	}

	w := lx.NewWalker(baseRules, overrideRules)
	w.IgnoreEnabled = !runCfg.NoIgnore

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

func collectURLArchiveTreePaths(ctx context.Context, rawPath string, showHidden bool, includes []string) []string {
	tempPath, cleanup, err := lx.DownloadURLToTempFile(ctx, rawPath)
	if err != nil {
		slog.Debug("Failed to download URL archive for tree", "url", rawPath, "error", err)
		return nil
	}
	defer cleanup()

	archiveWalker := newArchiveWalker(showHidden, false)
	paths, err := lx.ExpandArchivePaths(ctx, tempPath, rawPath, archiveWalker, includes)
	if err != nil {
		slog.Debug("Failed to expand URL archive for tree", "url", rawPath, "error", err)
		return nil
	}
	return paths
}

type treeNode struct {
	children   map[string]*treeNode
	childOrder []string
}

func newTreeNode() *treeNode {
	return &treeNode{children: make(map[string]*treeNode)}
}

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
	case 1:
		name := topLevel[0]
		child := root.children[name]
		displayName := name
		if len(child.children) > 0 {
			displayName += "/"
		}
		buf.WriteString(displayName + "\n")
		renderTreeChildren(&buf, child, "")
	default:
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
			return iIsDir
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
