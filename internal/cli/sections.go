package cli

import (
	"log/slog"
	"strconv"

	"github.com/rasros/lx/pkg/lx"
)

// Section holds all state and actions for one logical section of file operations.
// A new section begins whenever a CmdInterleaved op appears after file actions,
// resetting all interleaved state (RunCfg, Includes, Excludes) to defaults before
// the new flag takes effect.
type Section struct {
	RunCfg   lx.RunnerConfig
	Includes []string
	Excludes []string
	Ops      []Op // CmdAction ops in stream order

	// Populated by precomputeTrees; keyed by index within Ops.
	treeStrings map[int]string // tree/tree-only op index → ASCII tree string
	skipFileOps map[int]bool   // FILE/file op index → suppress content (tree-only mode)
}

// parseSections splits a flat list of ops into sections.
// reorderTrailingOps should be applied to ops before calling this.
func parseSections(ops []Op, defaultRunCfg lx.RunnerConfig) []Section {
	var sections []Section
	current := Section{RunCfg: defaultRunCfg}
	hasFiles := false

	for _, op := range ops {
		switch op.Type {
		case CmdInterleaved:
			if hasFiles {
				// Section boundary: flush the current section and start a fresh one.
				sections = append(sections, current)
				current = Section{RunCfg: defaultRunCfg}
				hasFiles = false
			}
			applyInterleaved(op, &current)
		case CmdAction:
			if op.Action == "FILE" || op.Action == "file" {
				hasFiles = true
			}
			current.Ops = append(current.Ops, op)
		}
	}
	if len(current.Ops) > 0 {
		sections = append(sections, current)
	}
	return sections
}

// applyInterleaved updates a section's settings from a single CmdInterleaved op.
func applyInterleaved(op Op, s *Section) {
	switch op.Action {
	case "head":
		val, _ := strconv.Atoi(op.Value)
		slog.Debug("Setting head limit", "value", val)
		s.RunCfg.Head, s.RunCfg.Tail = val, 0
	case "tail":
		val, _ := strconv.Atoi(op.Value)
		slog.Debug("Setting tail limit", "value", val)
		s.RunCfg.Tail, s.RunCfg.Head = val, 0
	case "lines":
		val, _ := strconv.Atoi(op.Value)
		slog.Debug("Setting lines limit", "value", val)
		s.RunCfg.Head, s.RunCfg.Tail = (val+1)/2, val/2
	case "line-numbers":
		slog.Debug("Enabling line numbers")
		s.RunCfg.LineNumbers = true
	case "functions":
		slog.Debug("Enabling function skeleton")
		s.RunCfg.SkeletonFunctions = true
	case "types":
		slog.Debug("Enabling type skeleton")
		s.RunCfg.SkeletonTypes = true
	case "include":
		slog.Debug("Adding include filter", "pattern", op.Value)
		s.Includes = append(s.Includes, op.Value)
	case "exclude":
		slog.Debug("Adding exclude filter", "pattern", op.Value)
		s.Excludes = append(s.Excludes, op.Value)
	}
}
