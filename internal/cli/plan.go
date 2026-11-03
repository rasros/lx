package cli

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/rasros/lx/pkg/lx"
)

// Section is a run of items rendered with one set of options.
type Section struct {
	Render   lx.RenderOptions
	Discover lx.DiscoverOptions
	Items    []item

	// bareTrees names, a group per entry, the tree options that cover no
	// files, for a warning once the inputs are collected.
	bareTrees []string
}

// item is one thing a section renders, in command-line order.
type item interface {
	// opAttrs returns the action and value of the option the item came
	// from, for the debug log.
	opAttrs() (action, value string)
}

// fileItem is a path, URL or archive named as input, or piped on stdin.
type fileItem struct {
	arg string
	// piped marks a path read from a file list on stdin.
	piped bool
	// treeOnly leaves its content out: a --tree-only stands in for it.
	treeOnly bool
	// files and failures are what collecting arg found.
	files    []lx.InputFile
	failures []lx.Failure
}

// stdinItem is "-": the content of stdin, read as one file.
type stdinItem struct{ treeOnly bool }

// treeItem is a -T or -t, over the file items beside it up to the next -t
// or the end of the section.
type treeItem struct {
	// op is the option, for warnings.
	op     Op
	inputs []*fileItem
}

// headingItem is a -s title.
type headingItem struct{ title string }

// promptItem is a -p prompt.
type promptItem struct{ text string }

// promptFileItem is a -P prompt, named by ref and found at path. A regular
// file is read before inputs are collected, so it fails first; other files,
// such as /dev/stdin, are read in turn while rendering.
type promptFileItem struct {
	ref, path, text string
	read            bool
}

// metaItem is a --system-context block.
type metaItem struct{}

func (f *fileItem) opAttrs() (string, string)       { return "FILE", f.arg }
func (*stdinItem) opAttrs() (string, string)        { return "FILE", "-" }
func (t *treeItem) opAttrs() (string, string)       { return t.op.Action, t.op.Value }
func (h *headingItem) opAttrs() (string, string)    { return "section", h.title }
func (p *promptItem) opAttrs() (string, string)     { return "prompt", p.text }
func (p *promptFileItem) opAttrs() (string, string) { return "prompt-file", p.ref }
func (*metaItem) opAttrs() (string, string)         { return "system-context", "true" }

// newItem returns the item of an action op.
func newItem(op Op) item {
	switch op.Action {
	case "FILE":
		if op.Value == "-" {
			return &stdinItem{}
		}
		return &fileItem{arg: op.Value, piped: op.Piped}
	case "tree", "tree-only":
		return &treeItem{op: op}
	case "section":
		return &headingItem{title: op.Value}
	case "prompt":
		return &promptItem{text: op.Value}
	case "prompt-file":
		return &promptFileItem{ref: op.Value}
	case "system-context":
		return &metaItem{}
	}
	panic("cli: unknown action " + op.Action)
}

// compile splits ops into sections of items, each with its own interleaved
// options. An interleaved option after files, or a -s after files, starts a
// new section with the options reset. Options after files that no file
// follows before the next -s or the end apply back to those files, with a
// warning, whatever prompts or trees come between. Options with no file
// before them in their section carry on into the section a -s starts, so
// options before the first file belong to it however many prompts or -s
// come first; options no file follows at all are reported.
func compile(log *slog.Logger, ops []Op, defaultRender lx.RenderOptions) []Section {
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
			it := newItem(op)
			switch it.(type) {
			case *fileItem, *stdinItem:
				hasFiles, afterFiles = true, false
				pending = nil
			case *headingItem:
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
				if len(current.Items) > 0 {
					sections = append(sections, current)
				}
				current, hasFiles, afterFiles = next, false, false
			}
			current.Items = append(current.Items, it)
		}
	}
	switch {
	case afterFiles:
		applyTrailing(log, pending, &sections[len(sections)-1])
	case !hasFiles && len(pending) > 0:
		log.Warn("Interleaved options apply to no files", "flags", flagNames(pending))
	}
	if len(current.Items) > 0 {
		sections = append(sections, current)
	}
	for i := range sections {
		sections[i].groupTrees()
	}
	return sections
}

// groupTrees gives each tree the file items it covers: those beside it up to
// the next --tree-only or the end of s. A --tree-only also leaves the content
// of the items it covers out; the items before it keep theirs.
func (s *Section) groupTrees() {
	var files []*fileItem
	var stdins []*stdinItem
	var trees []*treeItem
	treeOnly := false

	flush := func() {
		if len(trees) > 0 && len(files)+len(stdins) == 0 {
			ops := make([]Op, len(trees))
			for i, t := range trees {
				ops[i] = t.op
			}
			s.bareTrees = append(s.bareTrees, flagNames(ops))
		}
		for _, t := range trees {
			t.inputs = files
		}
		if treeOnly {
			for _, f := range files {
				f.treeOnly = true
			}
			for _, f := range stdins {
				f.treeOnly = true
			}
		}
		files, stdins, trees, treeOnly = nil, nil, nil, false
	}

	for _, it := range s.Items {
		switch it := it.(type) {
		case *treeItem:
			if it.op.Action == "tree-only" {
				flush()
				treeOnly = true
			}
			trees = append(trees, it)
		case *fileItem:
			files = append(files, it)
		case *stdinItem:
			stdins = append(stdins, it)
		}
	}
	flush()
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
