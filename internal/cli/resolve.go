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
			if op.Action != "FILE" {
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
	// gitExcludes are git's global excludes, which apply only inside a git
	// repository.
	gitExcludes []string
	outPath     string
	debug       bool
	cleanups    *[]func()
	// rules caches the ignore rules every walk of the run loads, so file
	// arguments sharing directories read their ignore files once. Nil gives
	// each walk its own.
	rules *lx.RuleCache
}

// resolveOp discovers every input one file argument refers to. The content pass
// and the tree builder both consume the result, so the filesystem is walked once
// and the two cannot disagree about what a path means.
//
// Stdin ("-") resolves to nothing: it is a buffer, not a discoverable path, and
// it can only be read once. The content pass handles it directly.
func resolveOp(ctx context.Context, op Op, rc resolveContext) (files []lx.InputFile, unresolved int) {
	rawPath := op.Value
	if rawPath == "-" {
		return nil, 0
	}
	forceExpand := false
	if rewritten, ok := lx.RewriteRepoURL(rawPath); ok {
		slog.Debug("Rewrote repo URL to archive", "from", rawPath, "to", rewritten)
		rawPath = rewritten
		forceExpand = true
	}

	if lx.IsHTTPURL(rawPath) {
		return resolveURL(ctx, rawPath, forceExpand, rc)
	}

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		slog.Error("Failed to resolve absolute path", "path", rawPath, "error", err)
		return nil, 1
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		slog.Error("Failed to stat path", "path", absPath, "error", err)
		return nil, 1
	}

	fsys, walkRoot, displayPrefix := walkTarget(rawPath, absPath, stat.IsDir())
	return walkInputs(ctx, walkTargetArgs{
		fsys:          fsys,
		walkRoot:      walkRoot,
		displayPrefix: displayPrefix,
		absPath:       absPath,
		rootIsDir:     stat.IsDir(),
	}, rc), 0
}

// resolveURL fetches a named URL. Like a named path it is never filtered
// itself; an archive's entries are.
func resolveURL(ctx context.Context, rawPath string, forceExpand bool, rc resolveContext) ([]lx.InputFile, int) {
	section := rc.section

	if (section.RunCfg.ExpandArchives || forceExpand) && lx.IsHTTPArchiveURL(rawPath) {
		tempPath, cleanup, err := lx.DownloadURLToTempFile(ctx, rawPath)
		if err != nil {
			slog.Error("Failed to download URL archive", "url", rawPath, "error", err)
			return nil, 0
		}
		*rc.cleanups = append(*rc.cleanups, cleanup)

		return expandArchive(ctx, tempPath, rawPath, rc, "Ignored in URL archive"), 0
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
// displayed. A file is opened as walkRoot within fsys. As in ripgrep, a named
// path is never filtered itself: a file is always included and a directory
// always walked, whatever the ignore rules and the -i and -e filters say.
func walkTarget(rawPath, absPath string, isDir bool) (fsys fs.FS, walkRoot, displayPrefix string) {
	if isDir {
		return os.DirFS(absPath), ".", filepath.Clean(rawPath)
	}
	rawPathClean := filepath.Clean(rawPath)
	if !filepath.IsAbs(rawPathClean) && !strings.HasPrefix(rawPathClean, "..") {
		return os.DirFS("."), filepath.ToSlash(rawPathClean), ""
	}
	return os.DirFS(filepath.Dir(absPath)), filepath.Base(absPath), filepath.Dir(rawPathClean)
}

type walkTargetArgs struct {
	fsys          fs.FS
	walkRoot      string
	displayPrefix string
	absPath       string
	rootIsDir     bool
}

func walkInputs(ctx context.Context, t walkTargetArgs, rc resolveContext) []lx.InputFile {
	section := rc.section
	includeSpecs := lx.CompileSpecs(section.Includes)

	ignoreEnabled := !section.RunCfg.NoIgnore

	var baseRules, gitExcludes, overrideRules []string
	if ignoreEnabled {
		baseRules = append(baseRules, rc.ignoreRules...)
		gitExcludes = append(gitExcludes, rc.gitExcludes...)
	}
	overrideRules = append(overrideRules, section.Excludes...)

	slog.Debug("Initializing Walker",
		"walk_root", t.walkRoot,
		"base_rules_count", len(baseRules),
		"override_rules_count", len(overrideRules),
	)

	opts := lx.WalkerOptions{
		Base:              baseRules,
		GitExcludes:       gitExcludes,
		Override:          overrideRules,
		IgnoreEnabled:     ignoreEnabled,
		SkipHidden:        !section.RunCfg.ShowHidden,
		FollowDirSymlinks: section.RunCfg.FollowDirSymlinks,
		SkipFileSymlinks:  section.RunCfg.SkipFileSymlinks,
		Cache:             rc.rules,
	}
	if rc.debug {
		opts.OnIgnore = func(p, reason string) {
			slog.Debug("Ignored", "path", p, "reason", reason)
		}
	}
	walker := lx.NewWalker(opts)

	var files []lx.InputFile

	err := walker.WalkPath(t.absPath, func(path string, d fs.DirEntry, err error) error {
		if !t.rootIsDir {
			// The walker names a file root by its base name; t.fsys
			// opens it as walkRoot.
			path = t.walkRoot
		}
		if err != nil {
			slog.Warn("Error accessing path during walk", "path", path, "error", err)
			return nil
		}
		if d.IsDir() {
			if len(includeSpecs) > 0 && path != "." && !lx.CouldMatchAnyDescendant(includeSpecs, path) {
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
			files = append(files, expandArchive(ctx, archiveAbsPath, effectivePath, rc, "Ignored in archive")...)
			return nil
		}

		if t.rootIsDir && len(includeSpecs) > 0 && !lx.IsMatchAnyCompiled(includeSpecs, path) {
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
		// A walked symlink reports the link's own size (its target path
		// length); the content reader is bounded by Size, so use the target's.
		// Broken links keep the link info and fail later when opened.
		if info.Mode()&fs.ModeSymlink != 0 {
			if target, statErr := fs.Stat(t.fsys, path); statErr == nil {
				info = target
			}
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

func expandArchive(ctx context.Context, absPath, displayPath string, rc resolveContext, ignoreMsg string) []lx.InputFile {
	var onIgnore func(p, reason string)
	if rc.debug {
		onIgnore = func(p, reason string) {
			slog.Debug(ignoreMsg, "path", displayPath+"/"+p, "reason", reason)
		}
	}
	archiveWalker := newArchiveWalker(rc.section.Excludes, rc.section.RunCfg.ShowHidden, onIgnore)

	collector := &fileCollector{}
	if err := lx.ExpandArchive(ctx, absPath, displayPath, archiveWalker, rc.section.Includes, rc.outPath, collector); err != nil {
		slog.Error("Failed to expand archive", "path", displayPath, "error", err)
	}
	return collector.files
}
