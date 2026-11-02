package cli

import (
	"log/slog"

	"github.com/rasros/lx/pkg/lx"
)

// assignTrees gives each tree op the resolved files it covers: the file ops
// beside it up to the next section or tree-only boundary. The stream builds
// the tree from the ones it admits. A tree-only op also marks its file ops so
// their content is skipped.
func assignTrees(sections []Section) {
	for i := range sections {
		assignSectionTrees(&sections[i])
	}
}

func assignSectionTrees(g *Section) {
	var subFileIdxs []int
	var subTreeIdxs []int
	subTreeOnly := false

	flush := func() {
		if len(subTreeIdxs) > 0 && len(subFileIdxs) == 0 {
			slog.Warn("Tree options cover no files", "flags", flagNames(treeOps(g.Ops, subTreeIdxs)))
		}
		if len(subTreeIdxs) > 0 {
			var files []lx.InputFile
			for _, idx := range subFileIdxs {
				files = append(files, g.resolved[idx]...)
			}
			if g.treeFiles == nil {
				g.treeFiles = make(map[int][]lx.InputFile)
			}
			for _, ti := range subTreeIdxs {
				g.treeFiles[ti] = files
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
		case "FILE":
			subFileIdxs = append(subFileIdxs, oi)
		}
	}
	flush()
}

// treeOps returns the ops of g at idxs.
func treeOps(ops []Op, idxs []int) []Op {
	out := make([]Op, len(idxs))
	for i, idx := range idxs {
		out[i] = ops[idx]
	}
	return out
}
