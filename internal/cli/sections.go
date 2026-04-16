package cli

import (
	"log/slog"
	"strconv"

	"github.com/rasros/lx/pkg/lx"
)

type Section struct {
	RunCfg   lx.RunnerConfig
	Includes []string
	Excludes []string
	Ops      []Op

	treeStrings map[int]string
	skipFileOps map[int]bool
}

func parseSections(ops []Op, defaultRunCfg lx.RunnerConfig) []Section {
	var sections []Section
	current := Section{RunCfg: defaultRunCfg}
	hasFiles := false

	for _, op := range ops {
		switch op.Type {
		case CmdInterleaved:
			if hasFiles {
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
