// Package discover turns the paths and URLs given to lx into the inputs it
// renders, with the ignore rules, filters and archive expansion of the CLI.
package discover

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasros/lx/internal/osenv"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/walker"
)

// Options decide what Collect finds for a target.
type Options struct {
	// Globs filter what is found below a target, as ripgrep's -g globs do:
	// the last one to match a path decides it, ahead of ignore files and
	// hidden-file skipping. They match the path from the working directory
	// as the target was named, with the working directory stripped from
	// the front of an absolute one. An archive being expanded is matched as
	// a directory, and its entries as the paths below it, so
	// "pkg/a.zip/src/*.go" names entries of pkg/a.zip; the entries of a URL
	// archive are below the URL. A named target is never filtered by them,
	// nor by anything but OutPath.
	Globs []walker.Override

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
	// a named target it is cleaned, as a path a walk finds, and itself
	// filtered by Globs, as a walk from the working directory would filter
	// it: dropped when it or a directory above it is excluded, or, when some
	// -i glob must keep a file, it is a file none keeps.
	Piped bool
}

// Collector collects the inputs of the targets of one run. It caches the
// ignore rules its walks load, so targets sharing directories read their
// ignore files once. It is not safe for concurrent use.
type Collector struct {
	cache *walker.Cache
	cwd   string
	log   *slog.Logger
}

// NewCollector returns a Collector over the OS filesystem that resolves
// relative targets against cwd, or the process's working directory when cwd
// is empty, and logs to log, or slog.Default() when log is nil.
func NewCollector(cwd string, log *slog.Logger) *Collector {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if log == nil {
		log = slog.Default()
	}
	return newCollector(cwdHost{wd: cwd}, cwd, log)
}

func newCollector(host walker.Host, cwd string, log *slog.Logger) *Collector {
	return &Collector{cache: walker.NewCache(host, log), cwd: cwd, log: log}
}

// cwdHost is the OS seen from the working directory wd.
type cwdHost struct {
	walker.OSHost
	wd string
}

func (h cwdHost) Getwd() (string, error) { return h.wd, nil }

// abs returns p as an absolute path, resolved against the working directory.
func (c *Collector) abs(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(c.cwd, p)
}

// Collect returns the inputs target refers to: a file, every file below a
// directory, the entries of an archive, or a URL. As in ripgrep, a named
// target is never filtered itself, while the files below it are filtered by
// the ignore rules of the directories above and below it, and by opts.
// Problems with single entries are logged and the entries skipped; an error
// means target itself could not be resolved, or ctx was done. A URL that
// cannot be downloaded or parsed fails with a *URLError.
func (c *Collector) Collect(ctx context.Context, target string, opts Options) ([]sources.InputFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	forceExpand := false
	if rewritten, ok := sources.RewriteRepoURL(target); ok {
		c.log.Debug("Rewrote repo URL to archive", "from", target, "to", rewritten)
		target = rewritten
		forceExpand = true
	}
	if sources.IsHTTPURL(target) {
		return c.collectURL(ctx, target, opts, forceExpand)
	}

	if opts.Piped {
		// A named root is matched as written, as ripgrep matches it, but a
		// piped path stands for a path a walk finds, which has no "//",
		// "./" or "..".
		target = filepath.Clean(target)
	}
	absPath := c.abs(target)
	stat, err := os.Stat(absPath)
	w := c.walker(ctx, opts)
	if opts.Piped && !w.Accepts(target, err == nil && (stat.IsDir() || expands(opts, target))) {
		c.log.Debug("Skipping piped path due to filters", "path", target)
		return nil, nil
	}
	if err != nil {
		// The output file is never collected, even named before it exists.
		if c.isOutput(target, opts.OutPath) {
			return nil, nil
		}
		return nil, err
	}
	return c.walk(ctx, w, newRoot(c.cwd, target, absPath, stat.IsDir()), opts)
}

// expands reports whether the file at path is an archive opts expand, which
// the globs match as a directory.
func expands(opts Options, path string) bool {
	return opts.ExpandArchives && sources.IsArchivePath(path)
}

// UserIgnoreRules reads the ignore files lx applies everywhere: lx's own,
// in the user config directory, and git's global excludes. It finds them
// through env, which looks variables up as os.LookupEnv does, and logs to
// log, or slog.Default() when log is nil.
func UserIgnoreRules(env osenv.Lookup, log *slog.Logger) (ignoreRules, gitExcludes []string) {
	if log == nil {
		log = slog.Default()
	}
	if configDir, err := osenv.Config(env); err == nil {
		ignoreRules = readIgnorePatterns(log, filepath.Join(configDir, "lx", "ignore"))
	}
	// Git looks in the XDG location on every system.
	if xdgConfig := osenv.XDGConfigHome(env); xdgConfig != "" {
		gitExcludes = readIgnorePatterns(log, filepath.Join(xdgConfig, "git", "ignore"))
	}
	return ignoreRules, gitExcludes
}

// readIgnorePatterns returns the patterns in the ignore file at p, if any.
func readIgnorePatterns(log *slog.Logger, p string) []string {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	log.Debug("Loaded global ignore file", "path", p)
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// collectURL fetches a named URL. Like a named path it is never filtered
// itself; an archive's entries are.
func (c *Collector) collectURL(ctx context.Context, rawURL string, opts Options, forceExpand bool) ([]sources.InputFile, error) {
	if (opts.ExpandArchives || forceExpand) && sources.IsHTTPArchiveURL(rawURL) {
		tempPath, cleanup, err := sources.DownloadURLToTempFile(ctx, rawURL)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, &URLError{URL: rawURL, Download: true, Err: err}
		}
		// Expansion reads the entries into memory.
		defer cleanup()
		return c.expandArchive(ctx, tempPath, rawURL, opts, strings.TrimSuffix(rawURL, "/")+"/", "Ignored in URL archive")
	}

	urlFile, err := sources.NewURLInputFile(rawURL)
	if err != nil {
		return nil, &URLError{URL: rawURL, Err: err}
	}
	c.log.Debug("URL accepted", "url", urlFile.Path)
	return []sources.InputFile{urlFile}, nil
}

// root is a local target: the filesystem its files are opened in and how
// their paths are displayed.
type root struct {
	rawPath       string
	fsys          fs.FS
	walkRoot      string
	displayPrefix string
	absPath       string
	isDir         bool
}

// newRoot decides which filesystem a target's files are opened in and how
// their paths are displayed. A file is opened as walkRoot within fsys.
func newRoot(cwd, rawPath, absPath string, isDir bool) root {
	r := root{rawPath: rawPath, absPath: absPath, isDir: isDir}
	switch rawPathClean := filepath.Clean(rawPath); {
	case isDir:
		r.fsys, r.walkRoot, r.displayPrefix = os.DirFS(absPath), ".", rawPathClean
	case !filepath.IsAbs(rawPathClean) && !strings.HasPrefix(rawPathClean, ".."):
		r.fsys, r.walkRoot = os.DirFS(cwd), filepath.ToSlash(rawPathClean)
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

func (c *Collector) debugEnabled(ctx context.Context) bool {
	return c.log.Enabled(ctx, slog.LevelDebug)
}

// walker returns the walker of a target's files.
func (c *Collector) walker(ctx context.Context, opts Options) *walker.Walker {
	var baseRules, gitExcludes []string
	if !opts.NoIgnore {
		baseRules = opts.IgnoreRules
		gitExcludes = opts.GitExcludes
	}
	wopts := walker.Options{
		Base:              baseRules,
		GitExcludes:       gitExcludes,
		Overrides:         opts.Globs,
		IgnoreEnabled:     !opts.NoIgnore,
		SkipHidden:        !opts.ShowHidden,
		FollowDirSymlinks: opts.FollowDirSymlinks,
		SkipFileSymlinks:  opts.SkipFileSymlinks,
		Cache:             c.cache,
	}
	if opts.ExpandArchives {
		wopts.MatchAsDir = sources.IsArchivePath
	}
	if c.debugEnabled(ctx) {
		wopts.OnIgnore = func(p, reason string) {
			c.log.Debug("Ignored", "path", p, "reason", reason)
		}
	}
	return walker.New(wopts)
}

func (c *Collector) walk(ctx context.Context, w *walker.Walker, r root, opts Options) ([]sources.InputFile, error) {
	c.log.Debug("Walking target", "root", r.rawPath, "globs", len(opts.Globs))
	anchor := c.cache.Anchor(r.rawPath)

	var files []sources.InputFile
	err := w.WalkPath(r.rawPath, func(path string, d fs.DirEntry, err error) error {
		if !r.isDir {
			// The walker names a file root by its base name; r.fsys opens it
			// as walkRoot.
			path = r.walkRoot
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			c.log.Warn("Error accessing path during walk", "path", path, "error", err)
			return nil
		}
		if d.IsDir() {
			return nil
		}

		display := r.display(path)
		if expands(opts, path) {
			// The entries of an archive are matched as the paths below it,
			// whether it was found in the walk or named.
			rel := "."
			if r.isDir {
				rel = path
			}
			entries, err := c.expandArchive(ctx, r.abs(path), display, opts, anchor.Path(rel)+"/", "Ignored in archive")
			files = append(files, entries...)
			return err
		}
		if c.isOutput(display, opts.OutPath) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			c.log.Error("Failed to stat file in walk", "path", path, "error", err)
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

		c.log.Debug("File accepted by walker", "path", f.Path, "size", f.Size)
		files = append(files, f)
		return nil
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		c.log.Error("Walker traversal failed", "error", err)
	}
	c.log.Debug("Walker finished", "root", r.walkRoot, "files_matched", len(files))
	return files, nil
}

// expandArchive returns the entries of the archive at absPath, displayed
// below displayPath, that opts keep. The globs match ovPrefix before each
// entry's path in the archive. Its only error is ctx's.
func (c *Collector) expandArchive(ctx context.Context, absPath, displayPath string, opts Options, ovPrefix, ignoreMsg string) ([]sources.InputFile, error) {
	wopts := walker.Options{
		Overrides:      opts.Globs,
		OverridePrefix: ovPrefix,
		SkipHidden:     !opts.ShowHidden,
		Log:            c.log,
	}
	if c.debugEnabled(ctx) {
		wopts.OnIgnore = func(p, reason string) {
			c.log.Debug(ignoreMsg, "path", displayPath+"/"+p, "reason", reason)
		}
	}
	files, err := sources.ExpandArchive(ctx, absPath, displayPath, walker.New(wopts), c.log)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		c.log.Error("Failed to expand archive", "path", displayPath, "error", err)
	}
	return files, nil
}

// isOutput reports whether path, relative to the working directory, is the
// file output is written to, outPath, which is never collected, even when
// named.
func (c *Collector) isOutput(path, outPath string) bool {
	if outPath == "" {
		return false
	}
	if c.abs(path) == outPath {
		c.log.Warn("Skipping output file to avoid infinite recursion", "path", path)
		return true
	}
	return false
}

// URLError reports a URL target that could not be downloaded, when Download
// is set, or not parsed.
type URLError struct {
	URL      string
	Download bool
	Err      error
}

func (e *URLError) Error() string {
	if e.Download {
		return "download " + e.URL + ": " + e.Err.Error()
	}
	return "parse " + e.URL + ": " + e.Err.Error()
}

func (e *URLError) Unwrap() error { return e.Err }
