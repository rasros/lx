package cli

import (
	"log/slog"
	"strconv"
	"strings"

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
	var trailingOps []Op

	for _, op := range ops {
		switch op.Type {
		case CmdInterleaved:
			if hasFiles {
				sections = append(sections, current)
				current = Section{RunCfg: defaultRunCfg}
				hasFiles = false
				trailingOps = nil
			}
			applyInterleaved(op, &current)
			trailingOps = append(trailingOps, op)
		case CmdAction:
			if op.Action == "FILE" || op.Action == "file" {
				hasFiles = true
				trailingOps = nil
			}
			current.Ops = append(current.Ops, op)
		}
	}

	if len(current.Ops) > 0 {
		sections = append(sections, current)
	} else if len(trailingOps) > 0 && len(sections) > 0 {
		names := make([]string, 0, len(trailingOps))
		for _, op := range trailingOps {
			desc := "--" + op.Action
			if op.Value != "" && op.Value != "true" {
				desc += "=" + strconv.Quote(op.Value)
			}
			names = append(names, desc)
		}
		slog.Warn("Trailing interleaved options detected; applying to preceding files",
			"flags", strings.Join(names, ", "))
		last := &sections[len(sections)-1]
		for _, op := range trailingOps {
			applyInterleaved(op, last)
		}
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
	case "max-size":
		val, _ := parseSizeLimit(op.Value)
		slog.Debug("Setting max file size", "bytes", val)
		s.RunCfg.MaxSize = val
	case "line-numbers":
		slog.Debug("Enabling line numbers")
		s.RunCfg.LineNumbers = true
	case "functions":
		slog.Debug("Enabling function skeleton")
		s.RunCfg.SkeletonFunctions = true
	case "types":
		slog.Debug("Enabling type skeleton")
		s.RunCfg.SkeletonTypes = true
	case "documents":
		slog.Debug("Enabling document extraction")
		s.RunCfg.ExtractDocuments = true
	case "expand":
		slog.Debug("Enabling archive expansion")
		s.RunCfg.ExpandArchives = true
	case "hidden":
		slog.Debug("Enabling show hidden files")
		s.RunCfg.ShowHidden = true
	case "follow":
		slog.Debug("Enabling follow dir symlinks")
		s.RunCfg.FollowDirSymlinks = true
	case "no-links":
		slog.Debug("Enabling skip file symlinks")
		s.RunCfg.SkipFileSymlinks = true
	case "no-ignore":
		slog.Debug("Disabling gitignore filtering")
		s.RunCfg.NoIgnore = true
	case "include":
		slog.Debug("Adding include filter", "pattern", op.Value)
		s.Includes = append(s.Includes, op.Value)
	case "exclude":
		slog.Debug("Adding exclude filter", "pattern", op.Value)
		s.Excludes = append(s.Excludes, op.Value)
	}
}
