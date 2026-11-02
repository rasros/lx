// Package discover turns the paths and URLs given to lx into the inputs it
// renders, with the ignore rules, filters and archive expansion of the CLI.
package discover

import (
	"context"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/walker"
)

// Options decide what Collect finds for a target.
type Options struct {
	// Includes keeps only the files below a walked directory that match one
	// of these globs, relative to it. The entries of an archive found in the
	// walk are matched the same way, as files below the archive's path; those
	// of a named archive relative to the archive, as for a named directory.
	// A named file is never filtered by them, nor by anything but OutPath.
	Includes []string
	// Excludes drops what matches these patterns, applied as ignore rules
	// after every ignore file, relative to the walked directory or archive.
	Excludes []string

	// IgnoreRules apply everywhere, below every ignore file, and GitExcludes,
	// git's global excludes, inside a repository. UserIgnoreRules reads the
	// ones lx uses.
	IgnoreRules []string
	GitExcludes []string
	// NoIgnore skips ignore files and the rules above.
	NoIgnore bool

	ShowHidden        bool
	FollowDirSymlinks bool
	SkipFileSymlinks  bool
	// ExpandArchives collects the files inside archives instead of the
	// archives themselves.
	ExpandArchives bool

	// OutPath is an absolute path never collected, such as the file output
	// is written to.
	OutPath string
	// Piped marks a target taken from a list of paths, as piped to lx. Unlike
	// a named target it is itself filtered by Includes and Excludes, matched
	// relative to the working directory.
	Piped bool
}

// Collector collects the inputs of the targets of one run. It caches the
// ignore rules its walks load, so targets sharing directories read their
// ignore files once. It is not safe for concurrent use.
type Collector struct {
	cache *walker.Cache
}

// NewCollector returns a Collector over the OS filesystem.
func NewCollector() *Collector {
	return &Collector{cache: walker.NewCache(nil)}
}

// Collect returns the inputs target refers to: a file, every file below a
// directory, the entries of an archive, or a URL. As in ripgrep, a named
// target is never filtered itself, while the files below it are filtered by
// the ignore rules of the directories above and below it, and by opts.
// Problems with single entries are logged and the entries skipped; an error
// means target itself could not be resolved, or ctx was done. A URL that
// cannot be fetched or parsed fails with a *url.Error whose Op is "download"
// or "parse".
func (c *Collector) Collect(ctx context.Context, target string, opts Options) ([]sources.InputFile, error) {
	sel := newSelector(opts)
	forceExpand := false
	if rewritten, ok := sources.RewriteRepoURL(target); ok {
		slog.Debug("Rewrote repo URL to archive", "from", target, "to", rewritten)
		target = rewritten
		forceExpand = true
	}
	if sources.IsHTTPURL(target) {
		return collectURL(ctx, target, opts, sel, forceExpand)
	}

	absPath, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(absPath)
	if opts.Piped && !sel.AcceptRoot(cwdRelative(target, absPath), err == nil && stat.IsDir()) {
		slog.Debug("Skipping piped path due to filters", "path", target)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c.walk(ctx, newRoot(target, absPath, stat.IsDir()), opts, sel)
}

// UserIgnoreRules reads the ignore files lx applies everywhere: lx's own,
// in the user config directory, and git's global excludes.
func UserIgnoreRules() (ignoreRules, gitExcludes []string) {
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	ignoreRules = readIgnorePatterns(filepath.Join(configDir, "lx", "ignore"))

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" && home != "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	if xdgConfig != "" {
		gitExcludes = readIgnorePatterns(filepath.Join(xdgConfig, "git", "ignore"))
	}
	return ignoreRules, gitExcludes
}

// readIgnorePatterns returns the patterns in the ignore file at p, if any.
func readIgnorePatterns(p string) []string {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	slog.Debug("Loaded global ignore file", "path", p)
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// cwdRelative returns absPath relative to the working directory, or rawPath
// when it is outside it.
func cwdRelative(rawPath, absPath string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, absPath); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filepath.Clean(rawPath))
}

// collectURL fetches a named URL. Like a named path it is never filtered
// itself; an archive's entries are.
func collectURL(ctx context.Context, rawURL string, opts Options, sel selector, forceExpand bool) ([]sources.InputFile, error) {
	if (opts.ExpandArchives || forceExpand) && sources.IsHTTPArchiveURL(rawURL) {
		tempPath, cleanup, err := sources.DownloadURLToTempFile(ctx, rawURL)
		if err != nil {
			return nil, &url.Error{Op: "download", URL: rawURL, Err: err}
		}
		// Expansion reads the entries into memory.
		defer cleanup()
		return expandArchive(ctx, tempPath, rawURL, opts, sel, "Ignored in URL archive"), nil
	}

	urlFile, err := sources.NewURLInputFile(rawURL)
	if err != nil {
		return nil, &url.Error{Op: "parse", URL: rawURL, Err: err}
	}
	slog.Debug("URL accepted", "url", urlFile.Path)
	return []sources.InputFile{urlFile}, nil
}

// root is a local target: the filesystem its files are opened in and how
// their paths are displayed.
type root struct {
	fsys          fs.FS
	walkRoot      string
	displayPrefix string
	absPath       string
	isDir         bool
}

// newRoot decides which filesystem a target's files are opened in and how
// their paths are displayed. A file is opened as walkRoot within fsys.
func newRoot(rawPath, absPath string, isDir bool) root {
	r := root{absPath: absPath, isDir: isDir}
	switch rawPathClean := filepath.Clean(rawPath); {
	case isDir:
		r.fsys, r.walkRoot, r.displayPrefix = os.DirFS(absPath), ".", rawPathClean
	case !filepath.IsAbs(rawPathClean) && !strings.HasPrefix(rawPathClean, ".."):
		r.fsys, r.walkRoot = os.DirFS("."), filepath.ToSlash(rawPathClean)
	default:
		r.fsys, r.walkRoot, r.displayPrefix = os.DirFS(filepath.Dir(absPath)), filepath.Base(absPath), filepath.Dir(rawPathClean)
	}
	return r
}

// display is how the file at path, relative to the walked root, is shown.
func (r root) display(path string) string {
	if r.isDir && path == "." {
		return r.displayPrefix
	}
	if r.displayPrefix == "" {
		return filepath.FromSlash(path)
	}
	return filepath.Join(r.displayPrefix, filepath.FromSlash(path))
}

// abs is the absolute path of the file at path, relative to the walked root.
func (r root) abs(path string) string {
	switch {
	case r.isDir:
		return filepath.Join(r.absPath, path)
	case r.displayPrefix != "":
		return filepath.Join(filepath.Dir(r.absPath), path)
	default:
		return r.absPath
	}
}

func debugEnabled(ctx context.Context) bool {
	return slog.Default().Enabled(ctx, slog.LevelDebug)
}

func (c *Collector) walk(ctx context.Context, r root, opts Options, sel selector) ([]sources.InputFile, error) {
	var baseRules, gitExcludes []string
	if !opts.NoIgnore {
		baseRules = opts.IgnoreRules
		gitExcludes = opts.GitExcludes
	}

	slog.Debug("Initializing Walker",
		"walk_root", r.walkRoot,
		"base_rules_count", len(baseRules),
		"override_rules_count", len(opts.Excludes),
	)

	wopts := walker.Options{
		Base:              baseRules,
		GitExcludes:       gitExcludes,
		Override:          opts.Excludes,
		IgnoreEnabled:     !opts.NoIgnore,
		SkipHidden:        !opts.ShowHidden,
		FollowDirSymlinks: opts.FollowDirSymlinks,
		SkipFileSymlinks:  opts.SkipFileSymlinks,
		Cache:             c.cache,
	}
	if debugEnabled(ctx) {
		wopts.OnIgnore = func(p, reason string) {
			slog.Debug("Ignored", "path", p, "reason", reason)
		}
	}

	var files []sources.InputFile
	err := walker.New(wopts).WalkPath(r.absPath, func(path string, d fs.DirEntry, err error) error {
		if !r.isDir {
			// The walker names a file root by its base name; r.fsys opens it
			// as walkRoot.
			path = r.walkRoot
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			slog.Warn("Error accessing path during walk", "path", path, "error", err)
			return nil
		}
		if d.IsDir() {
			if !sel.Descend(path) {
				return fs.SkipDir
			}
			return nil
		}

		display := r.display(path)
		if opts.ExpandArchives && sources.IsArchivePath(path) {
			// The entries of an archive found in the walk are matched as
			// files below its path; those of a named one as files below it.
			archiveSel := sel
			if r.isDir {
				archiveSel = sel.within(path)
			}
			files = append(files, expandArchive(ctx, r.abs(path), display, opts, archiveSel, "Ignored in archive")...)
			return nil
		}
		if r.isDir {
			if !sel.Accept(path, display) {
				return nil
			}
		} else if sel.isOutput(display) {
			// A named file is filtered by nothing else.
			return nil
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
			if target, statErr := fs.Stat(r.fsys, path); statErr == nil {
				info = target
			}
		}

		f := sources.NewInputFile(r.fsys, path, info)
		f.Path = display
		f.AbsPath = r.abs(path)

		slog.Debug("File accepted by walker", "path", f.Path, "size", f.Size)
		files = append(files, f)
		return nil
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		slog.Error("Walker traversal failed", "error", err)
	}
	slog.Debug("Walker finished", "root", r.walkRoot, "files_matched", len(files))
	return files, nil
}

// expandArchive returns the entries of the archive at absPath, displayed
// below displayPath, that opts and sel keep. Exclude patterns become override
// rules relative to the archive root, the same way they are relative to a
// walked directory.
func expandArchive(ctx context.Context, absPath, displayPath string, opts Options, sel selector, ignoreMsg string) []sources.InputFile {
	wopts := walker.Options{
		Override:   opts.Excludes,
		SkipHidden: !opts.ShowHidden,
	}
	if debugEnabled(ctx) {
		wopts.OnIgnore = func(p, reason string) {
			slog.Debug(ignoreMsg, "path", displayPath+"/"+p, "reason", reason)
		}
	}
	files, err := sources.ExpandArchive(ctx, absPath, displayPath, walker.New(wopts), sel)
	if err != nil {
		slog.Error("Failed to expand archive", "path", displayPath, "error", err)
	}
	return files
}
