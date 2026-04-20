package cli

import (
	"context"
	"io/fs"
	"log/slog"
	"net/url"
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
	includeSpecs := lx.CompileSpecs(includes)

	rawPath := op.Value
	isForced := op.Action == "file"

	if rawPath == "-" {
		return paths
	}

	if lx.IsHTTPURL(rawPath) {
		if runCfg.ExpandArchives && lx.IsHTTPArchiveURL(rawPath) {
			if !isForced && !lx.IsKept(rawPath, nil, excludes) {
				return paths
			}
			archiveIncludes := includes
			if isForced {
				archiveIncludes = nil
			}
			return collectURLArchiveTreePaths(ctx, rawPath, runCfg.ShowHidden, isForced, archiveIncludes)
		}
		if !isForced && !lx.IsKept(rawPath, includes, excludes) {
			return paths
		}
		return append(paths, rawPath)
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
		isExpandableArchive := runCfg.ExpandArchives && lx.IsArchivePath(rawPath)
		if !isForced && !isExpandableArchive && !lx.IsKept(rawPath, includes, excludes) {
			return paths
		}
		if !isForced && isExpandableArchive && !lx.IsKept(rawPath, nil, excludes) {
			return paths
		}

		rawPathClean := filepath.Clean(rawPath)
		effectivePath := filepath.ToSlash(rawPathClean)
		if filepath.IsAbs(rawPathClean) {
			effectivePath = filepath.ToSlash(absPath)
		}

		if isExpandableArchive {
			archiveWalker := newArchiveWalker(runCfg.ShowHidden, isForced)
			archiveIncludes := includes
			if isForced {
				archiveIncludes = nil
			}
			archivePaths, err := lx.ExpandArchivePaths(ctx, absPath, effectivePath, archiveWalker, archiveIncludes)
			if err != nil {
				slog.Debug("Failed to expand archive for tree", "path", rawPath, "error", err)
				return paths
			}
			return append(paths, archivePaths...)
		}
		return append(paths, rawPathClean)
	}

	fsys := os.DirFS(absPath)
	displayPrefix := filepath.Clean(rawPath)

	var baseRules, overrideRules []string
	if !runCfg.NoIgnore {
		baseRules = append(baseRules, globalIgnoreRules...)
	}
	if !isForced {
		overrideRules = append(overrideRules, excludes...)
	}

	w := lx.NewWalker(baseRules, overrideRules)
	w.IgnoreEnabled = !runCfg.NoIgnore
	w.SkipHidden = !runCfg.ShowHidden && !isForced

	_ = w.Walk(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if !isForced && len(includeSpecs) > 0 && path != "." && !lx.CouldMatchAnyDescendant(includeSpecs, path) {
				return fs.SkipDir
			}
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

		if runCfg.ExpandArchives && lx.IsArchivePath(path) {
			archiveAbsPath := filepath.Join(absPath, filepath.FromSlash(path))
			archiveWalker := newArchiveWalker(runCfg.ShowHidden, isForced)
			archiveIncludes := includes
			if isForced {
				archiveIncludes = nil
			}
			archivePaths, err := lx.ExpandArchivePaths(ctx, archiveAbsPath, effectivePath, archiveWalker, archiveIncludes)
			if err != nil {
				slog.Debug("Failed to expand archive for tree", "path", effectivePath, "error", err)
				return nil
			}
			paths = append(paths, archivePaths...)
			return nil
		}

		if !isForced && len(includeSpecs) > 0 {
			if !lx.IsMatchAnyCompiled(includeSpecs, path) {
				return nil
			}
		}

		paths = append(paths, effectivePath)
		return nil
	})

	return paths
}

func collectURLArchiveTreePaths(ctx context.Context, rawPath string, showHidden bool, isForced bool, includes []string) []string {
	tempPath, cleanup, err := lx.DownloadURLToTempFile(ctx, rawPath)
	if err != nil {
		slog.Debug("Failed to download URL archive for tree", "url", rawPath, "error", err)
		return nil
	}
	defer cleanup()

	archiveWalker := newArchiveWalker(showHidden, isForced)
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
		parts := splitTreePathParts(p)
		if len(parts) == 0 {
			continue
		}
		node := root
		for _, part := range parts {
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

func splitTreePathParts(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if u, err := url.Parse(raw); err == nil {
		scheme := strings.ToLower(u.Scheme)
		if (scheme == "http" || scheme == "https") && u.Host != "" {
			root := scheme + "://" + u.Host
			escaped := strings.TrimPrefix(u.EscapedPath(), "/")
			if escaped == "" {
				return []string{root}
			}
			parts := strings.Split(escaped, "/")
			if u.RawQuery != "" {
				last := len(parts) - 1
				parts[last] = parts[last] + "?" + u.RawQuery
			}
			return append([]string{root}, parts...)
		}
	}

	cleaned := filepath.ToSlash(filepath.Clean(raw))
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "" || cleaned == "." {
		return nil
	}

	allParts := strings.Split(cleaned, "/")
	parts := make([]string, 0, len(allParts))
	for _, part := range allParts {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
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
