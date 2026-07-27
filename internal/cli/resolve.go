package cli

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasros/lx/pkg/lx"
)

// fileCollector gathers expanded archive entries.
type fileCollector struct{ files []lx.InputFile }

func (c *fileCollector) Add(f lx.InputFile) { c.files = append(c.files, f) }

// resolveSections resolves every file op once, before rendering, so the tree
// builder and the content pass share one view of what each path refers to.
func resolveSections(ctx context.Context, sections []Section, rc resolveContext) int {
	unresolved := 0
	for si := range sections {
		rc.section = sections[si]
		for oi, op := range sections[si].Ops {
			if op.Action != "FILE" && op.Action != "file" {
				continue
			}
			files, missing := resolveOp(ctx, op, rc)
			unresolved += missing
			if len(files) == 0 {
				continue
			}
			if sections[si].resolved == nil {
				sections[si].resolved = make(map[int][]lx.InputFile)
			}
			sections[si].resolved[oi] = files
		}
	}
	return unresolved
}

// resolveContext carries the per-section state discovery depends on.
type resolveContext struct {
	section     Section
	ignoreRules []string
	outPath     string
	debug       bool
	cleanups    *[]func()
}

// resolveOp discovers every input one file argument refers to. The content pass
// and the tree builder both consume the result, so the filesystem is walked once
// and the two cannot disagree about what a path means.
//
// Stdin ("-") resolves to nothing: it is a buffer, not a discoverable path, and
// it can only be read once. The content pass handles it directly.
func resolveOp(ctx context.Context, op Op, rc resolveContext) (files []lx.InputFile, unresolved int) {
	section := rc.section
	rawPath := op.Value
	if rawPath == "-" {
		return nil, 0
	}
	isForced := op.Action == "file"

	forceExpand := false
	if rewritten, ok := lx.RewriteRepoURL(rawPath); ok {
		slog.Debug("Rewrote repo URL to archive", "from", rawPath, "to", rewritten)
		rawPath = rewritten
		forceExpand = true
	}

	if lx.IsHTTPURL(rawPath) {
		return resolveURL(ctx, rawPath, isForced, forceExpand, rc)
	}

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		slog.Error("Failed to resolve absolute path", "path", rawPath, "error", err)
		return nil, 1
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		if !isForced && !lx.IsKept(rawPath, section.Includes, section.Excludes) {
			slog.Debug("Skipping missing path due to filters", "path", rawPath)
			return nil, 0
		}
		slog.Error("Failed to stat path", "path", absPath, "error", err)
		return nil, 1
	}

	fsys, walkRoot, displayPrefix, ok := walkTarget(rawPath, absPath, stat.IsDir(), isForced, section)
	if !ok {
		return nil, 0
	}

	return walkInputs(ctx, walkTargetArgs{
		fsys:          fsys,
		walkRoot:      walkRoot,
		displayPrefix: displayPrefix,
		absPath:       absPath,
		rootIsDir:     stat.IsDir(),
		isForced:      isForced,
	}, rc), 0
}

func resolveURL(ctx context.Context, rawPath string, isForced, forceExpand bool, rc resolveContext) ([]lx.InputFile, int) {
	section := rc.section

	if (section.RunCfg.ExpandArchives || forceExpand) && lx.IsHTTPArchiveURL(rawPath) {
		if !isForced && !lx.IsKept(rawPath, nil, section.Excludes) {
			slog.Debug("Skipping URL archive due to exclude filter", "url", rawPath)
			return nil, 0
		}

		tempPath, cleanup, err := lx.DownloadURLToTempFile(ctx, rawPath)
		if err != nil {
			slog.Error("Failed to download URL archive", "url", rawPath, "error", err)
			return nil, 0
		}
		*rc.cleanups = append(*rc.cleanups, cleanup)

		return expandArchive(ctx, tempPath, rawPath, isForced, rc, "Ignored in URL archive"), 0
	}

	if !isForced && !lx.IsKept(rawPath, section.Includes, section.Excludes) {
		slog.Debug("Skipping URL due to filters", "url", rawPath)
		return nil, 0
	}

	urlFile, err := lx.NewURLInputFile(rawPath)
	if err != nil {
		slog.Error("Failed to create URL input", "url", rawPath, "error", err)
		return nil, 1
	}
	slog.Debug("URL accepted", "url", urlFile.Path)
	return []lx.InputFile{urlFile}, 0
}

// walkTarget decides which filesystem to walk and how discovered paths should be
// displayed, applying the filters that reject a path outright.
func walkTarget(rawPath, absPath string, isDir, isForced bool, section Section) (fsys fs.FS, walkRoot, displayPrefix string, ok bool) {
	if isDir {
		return os.DirFS(absPath), ".", filepath.Clean(rawPath), true
	}

	isExpandableArchive := section.RunCfg.ExpandArchives && lx.IsArchivePath(rawPath)
	if !isForced && !isExpandableArchive && !lx.IsKept(rawPath, section.Includes, section.Excludes) {
		slog.Debug("Skipping file due to filters", "path", rawPath)
		return nil, "", "", false
	}
	if !isForced && isExpandableArchive && !lx.IsKept(rawPath, nil, section.Excludes) {
		slog.Debug("Skipping archive due to exclude filter", "path", rawPath)
		return nil, "", "", false
	}

	rawPathClean := filepath.Clean(rawPath)
	if !filepath.IsAbs(rawPathClean) && !strings.HasPrefix(rawPathClean, "..") {
		return os.DirFS("."), filepath.ToSlash(rawPathClean), "", true
	}
	return os.DirFS(filepath.Dir(absPath)), filepath.Base(absPath), filepath.Dir(rawPathClean), true
}

type walkTargetArgs struct {
	fsys          fs.FS
	walkRoot      string
	displayPrefix string
	absPath       string
	rootIsDir     bool
	isForced      bool
}

func walkInputs(ctx context.Context, t walkTargetArgs, rc resolveContext) []lx.InputFile {
	section := rc.section
	includeSpecs := lx.CompileSpecs(section.Includes)

	var baseRules, overrideRules []string
	if !section.RunCfg.NoIgnore {
		baseRules = append(baseRules, rc.ignoreRules...)
	}
	if !t.isForced {
		overrideRules = append(overrideRules, section.Excludes...)
	}

	slog.Debug("Initializing Walker",
		"walk_root", t.walkRoot,
		"base_rules_count", len(baseRules),
		"override_rules_count", len(overrideRules),
		"is_forced", t.isForced,
	)

	walker := lx.NewWalker(baseRules, overrideRules)
	walker.IgnoreEnabled = !section.RunCfg.NoIgnore
	walker.SkipHidden = !section.RunCfg.ShowHidden && !t.isForced
	walker.FollowDirSymlinks = section.RunCfg.FollowDirSymlinks
	walker.SkipFileSymlinks = section.RunCfg.SkipFileSymlinks
	if rc.debug {
		walker.OnIgnore = func(p, reason string) {
			slog.Debug("Ignored", "path", p, "reason", reason)
		}
	}

	var files []lx.InputFile

	err := walker.Walk(t.fsys, t.walkRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("Error accessing path during walk", "path", path, "error", err)
			return nil
		}
		if d.IsDir() {
			if !t.isForced && len(includeSpecs) > 0 && path != "." && !lx.CouldMatchAnyDescendant(includeSpecs, path) {
				return fs.SkipDir
			}
			return nil
		}

		effectivePath := displayPath(path, t)

		if section.RunCfg.ExpandArchives && lx.IsArchivePath(path) {
			archiveAbsPath := t.absPath
			if t.rootIsDir {
				archiveAbsPath = filepath.Join(t.absPath, filepath.FromSlash(path))
			}
			files = append(files, expandArchive(ctx, archiveAbsPath, effectivePath, t.isForced, rc, "Ignored in archive")...)
			return nil
		}

		if !t.isForced && len(includeSpecs) > 0 && !lx.IsMatchAnyCompiled(includeSpecs, path) {
			slog.Debug("Ignored by include filter (-i)", "path", effectivePath)
			return nil
		}

		if rc.outPath != "" {
			if abs, _ := filepath.Abs(effectivePath); abs == rc.outPath {
				slog.Warn("Skipping output file to avoid infinite recursion", "path", effectivePath)
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			slog.Error("Failed to stat file in walk", "path", path, "error", err)
			return nil
		}

		f := lx.NewInputFile(t.fsys, path, info)
		f.Path = effectivePath
		f.AbsPath = inputAbsPath(path, t)

		slog.Debug("File accepted by walker", "path", f.Path, "size", f.Size)
		files = append(files, f)
		return nil
	})
	if err != nil {
		slog.Error("Walker traversal failed", "error", err)
	}
	slog.Debug("Walker finished", "root", t.walkRoot, "files_matched", len(files))

	return files
}

func displayPath(path string, t walkTargetArgs) string {
	if !t.rootIsDir {
		if t.displayPrefix != "" {
			return filepath.Join(t.displayPrefix, filepath.FromSlash(path))
		}
		return filepath.FromSlash(path)
	}
	if path == "." {
		return t.displayPrefix
	}
	return filepath.Join(t.displayPrefix, filepath.FromSlash(path))
}

func inputAbsPath(path string, t walkTargetArgs) string {
	if t.rootIsDir {
		return filepath.Join(t.absPath, path)
	}
	if t.displayPrefix != "" {
		return filepath.Join(filepath.Dir(t.absPath), path)
	}
	return t.absPath
}

func expandArchive(ctx context.Context, absPath, displayPath string, isForced bool, rc resolveContext, ignoreMsg string) []lx.InputFile {
	archiveWalker := newArchiveWalker(rc.section.RunCfg.ShowHidden, isForced)
	if rc.debug {
		archiveWalker.OnIgnore = func(p, reason string) {
			slog.Debug(ignoreMsg, "path", displayPath+"/"+p, "reason", reason)
		}
	}

	includes := rc.section.Includes
	if isForced {
		includes = nil
	}

	collector := &fileCollector{}
	if err := lx.ExpandArchive(ctx, absPath, displayPath, archiveWalker, includes, rc.outPath, collector); err != nil {
		slog.Error("Failed to expand archive", "path", displayPath, "error", err)
	}
	return collector.files
}
