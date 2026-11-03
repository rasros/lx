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
	failures    map[int][]lx.Failure
}

// parseSections splits ops into sections, each with its own interleaved
// options. An interleaved option after files, or a -s after files, starts a
// new section with the options reset. Options after files that no file
// follows before the next -s or the end apply back to those files, with a
// warning, whatever prompts or trees come between. Options with no file
// before them in their section carry on into the section a -s starts, so
// options before the first file belong to it however many prompts or -s
// come first; options no file follows at all are reported.
func parseSections(log *slog.Logger, ops []Op, defaultRender lx.RenderOptions) []Section {
	var sections []Section
	current := Section{Render: defaultRender}
	hasFiles := false
	// pending are the interleaved options no file has followed yet, and
	// afterFiles says they came right after files, which current was split
	// from.
	var pending []Op
	afterFiles := false

	for _, op := range ops {
		switch op.Type {
		case CmdInterleaved:
			if hasFiles {
				sections = append(sections, current)
				current = Section{Render: defaultRender}
				hasFiles, afterFiles = false, true
			}
			applyInterleaved(log, op, &current)
			pending = append(pending, op)
		case CmdAction:
			switch op.Action {
			case "FILE":
				hasFiles, afterFiles = true, false
				pending = nil
			case "section":
				next := Section{Render: defaultRender}
				switch {
				case afterFiles:
					applyTrailing(log, pending, &sections[len(sections)-1])
					pending = nil
				case !hasFiles:
					next.Render, next.Discover = current.Render, current.Discover
				default:
					pending = nil
				}
				if len(current.Ops) > 0 {
					sections = append(sections, current)
				}
				current, hasFiles, afterFiles = next, false, false
			}
			current.Ops = append(current.Ops, op)
		}
	}
	switch {
	case afterFiles:
		applyTrailing(log, pending, &sections[len(sections)-1])
	case !hasFiles && len(pending) > 0:
		log.Warn("Interleaved options apply to no files", "flags", flagNames(pending))
	}
	if len(current.Ops) > 0 {
		sections = append(sections, current)
	}
	return sections
}

// flagNames names ops as they could be written on the command line.
func flagNames(ops []Op) string {
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		desc := "--" + op.Action
		if op.Value != "" && op.Value != "true" {
			desc += "=" + strconv.Quote(op.Value)
		}
		names = append(names, desc)
	}
	return strings.Join(names, ", ")
}

// applyTrailing applies interleaved options that no file follows to the
// section of the files before them.
func applyTrailing(log *slog.Logger, ops []Op, s *Section) {
	log.Warn("Trailing interleaved options detected; applying to preceding files", "flags", flagNames(ops))
	for _, op := range ops {
		applyInterleaved(log, op, s)
	}
}

func applyInterleaved(log *slog.Logger, op Op, s *Section) {
	switch op.Action {
	case "head":
		val, _ := strconv.Atoi(op.Value)
		log.Debug("Setting head limit", "value", val)
		s.Render.Head, s.Render.Tail = val, 0
	case "tail":
		val, _ := strconv.Atoi(op.Value)
		log.Debug("Setting tail limit", "value", val)
		s.Render.Tail, s.Render.Head = val, 0
	case "lines":
		val, _ := strconv.Atoi(op.Value)
		log.Debug("Setting lines limit", "value", val)
		s.Render.Head, s.Render.Tail = (val+1)/2, val/2
	case "max-size":
		val, _ := parseSizeLimit(op.Value)
		log.Debug("Setting max file size", "bytes", val)
		s.Render.MaxSize = val
	case "line-numbers":
		log.Debug("Enabling line numbers")
		s.Render.LineNumbers = true
	case "functions":
		log.Debug("Enabling function skeleton")
		s.Render.SkeletonFunctions = true
	case "types":
		log.Debug("Enabling type skeleton")
		s.Render.SkeletonTypes = true
	case "documents":
		log.Debug("Enabling document extraction")
		s.Render.ExtractDocuments = true
	case "media":
		log.Debug("Enabling media metadata extraction")
		s.Render.ExtractMedia = true
	case "expand":
		log.Debug("Enabling archive expansion")
		s.Discover.ExpandArchives = true
	case "hidden":
		log.Debug("Enabling show hidden files")
		s.Discover.ShowHidden = true
	case "follow":
		log.Debug("Enabling follow dir symlinks")
		s.Discover.FollowDirSymlinks = true
	case "no-links":
		log.Debug("Enabling skip file symlinks")
		s.Discover.SkipFileSymlinks = true
	case "no-ignore":
		log.Debug("Disabling gitignore filtering")
		s.Discover.NoIgnore = true
	case "include":
		log.Debug("Adding include filter", "pattern", op.Value)
		s.Discover.Globs = append(s.Discover.Globs, lx.IncludeGlob(op.Value))
	case "exclude":
		log.Debug("Adding exclude filter", "pattern", op.Value)
		s.Discover.Globs = append(s.Discover.Globs, lx.ExcludeGlob(op.Value))
	}
}
