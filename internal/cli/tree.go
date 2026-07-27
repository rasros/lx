package cli

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

func precomputeTrees(groups []Section) {
	for i := range groups {
		computeSectionTrees(&groups[i])
	}
}

func computeSectionTrees(g *Section) {
	var subFileIdxs []int
	var subTreeIdxs []int
	subTreeOnly := false

	flush := func() {
		if len(subTreeIdxs) > 0 && len(subFileIdxs) > 0 {
			var paths []string
			for _, idx := range subFileIdxs {
				for _, f := range g.resolved[idx] {
					if overMaxSize(g.RunCfg, f.Size) {
						continue
					}
					paths = append(paths, f.Path)
				}
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
