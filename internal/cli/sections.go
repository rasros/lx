package cli

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/rasros/lx/pkg/lx"
)

type Section struct {
	Render   lx.RenderOptions
	Discover lx.DiscoverOptions
	Ops      []Op

	treeFiles   map[int][]lx.InputFile
	skipFileOps map[int]bool
	resolved    map[int][]lx.InputFile
}

// parseSections splits ops into sections, each with its own interleaved
// options. An interleaved option after files, or a -s, starts a new section
// with the options reset; options before the first file or -s belong to the
// first section. Options that no file follows before the next -s or the end
// apply to the files before them, with a warning.
func parseSections(ops []Op, defaultRender lx.RenderOptions) []Section {
	var sections []Section
	current := Section{Render: defaultRender}
	hasFiles := false
	var trailingOps []Op

	// closeTrailing ends the current section at a -s or the end of ops.
	closeTrailing := func() {
		if len(current.Ops) > 0 {
			sections = append(sections, current)
		} else if len(trailingOps) > 0 && len(sections) > 0 {
			applyTrailing(trailingOps, &sections[len(sections)-1])
		}
	}

	for _, op := range ops {
		switch op.Type {
		case CmdInterleaved:
			if hasFiles {
				sections = append(sections, current)
				current = Section{Render: defaultRender}
				hasFiles = false
				trailingOps = nil
			}
			applyInterleaved(op, &current)
			trailingOps = append(trailingOps, op)
		case CmdAction:
			switch {
			case op.Action == "section" && (len(current.Ops) > 0 || len(sections) > 0):
				closeTrailing()
				current = Section{Render: defaultRender}
				hasFiles = false
				trailingOps = nil
			case op.Action == "FILE":
				hasFiles = true
				trailingOps = nil
			}
			current.Ops = append(current.Ops, op)
		}
	}
	closeTrailing()
	return sections
}

// applyTrailing applies interleaved options that no file follows to the
// section of the files before them.
func applyTrailing(ops []Op, s *Section) {
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		desc := "--" + op.Action
		if op.Value != "" && op.Value != "true" {
			desc += "=" + strconv.Quote(op.Value)
		}
		names = append(names, desc)
	}
	slog.Warn("Trailing interleaved options detected; applying to preceding files",
		"flags", strings.Join(names, ", "))
	for _, op := range ops {
		applyInterleaved(op, s)
	}
}

func applyInterleaved(op Op, s *Section) {
	switch op.Action {
	case "head":
		val, _ := strconv.Atoi(op.Value)
		slog.Debug("Setting head limit", "value", val)
		s.Render.Head, s.Render.Tail = val, 0
	case "tail":
		val, _ := strconv.Atoi(op.Value)
		slog.Debug("Setting tail limit", "value", val)
		s.Render.Tail, s.Render.Head = val, 0
	case "lines":
		val, _ := strconv.Atoi(op.Value)
		slog.Debug("Setting lines limit", "value", val)
		s.Render.Head, s.Render.Tail = (val+1)/2, val/2
	case "max-size":
		val, _ := parseSizeLimit(op.Value)
		slog.Debug("Setting max file size", "bytes", val)
		s.Render.MaxSize = val
	case "line-numbers":
		slog.Debug("Enabling line numbers")
		s.Render.LineNumbers = true
	case "functions":
		slog.Debug("Enabling function skeleton")
		s.Render.SkeletonFunctions = true
	case "types":
		slog.Debug("Enabling type skeleton")
		s.Render.SkeletonTypes = true
	case "documents":
		slog.Debug("Enabling document extraction")
		s.Render.ExtractDocuments = true
	case "media":
		slog.Debug("Enabling media metadata extraction")
		s.Render.ExtractMedia = true
	case "expand":
		slog.Debug("Enabling archive expansion")
		s.Discover.ExpandArchives = true
	case "hidden":
		slog.Debug("Enabling show hidden files")
		s.Discover.ShowHidden = true
	case "follow":
		slog.Debug("Enabling follow dir symlinks")
		s.Discover.FollowDirSymlinks = true
	case "no-links":
		slog.Debug("Enabling skip file symlinks")
		s.Discover.SkipFileSymlinks = true
	case "no-ignore":
		slog.Debug("Disabling gitignore filtering")
		s.Discover.NoIgnore = true
	case "include":
		slog.Debug("Adding include filter", "pattern", op.Value)
		s.Discover.Globs = append(s.Discover.Globs, lx.IncludeGlob(op.Value))
	case "exclude":
		slog.Debug("Adding exclude filter", "pattern", op.Value)
		s.Discover.Globs = append(s.Discover.Globs, lx.ExcludeGlob(op.Value))
	}
}
