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
	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/walker"
)

// Options decide what Collect finds for a target.
type Options struct {
	// Globs filter what is found below a target, as ripgrep's -g globs do:
	// the last match decides, ahead of ignore files and hidden-file
	// skipping. They match the path as named from the working directory.
	// An expanded archive matches as a directory, so "pkg/a.zip/src/*.go"
	// names its entries, and a repository URL's files match from the
	// repository root. A named target is never filtered.
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
	// MaxSize, when positive, leaves larger archive entries unread so the
	// stream drops them. It should match RenderOptions.MaxSize.
	MaxSize int64
	// TempDir is where URL archives are downloaded to be expanded, or the
	// default directory for temporary files when empty.
	TempDir string

	// OutPath is an absolute path never collected, such as the file output
	// is written to.
	OutPath string
	// Piped marks a target read from a piped path list. Unlike a named
	// target it is cleaned and filtered by Globs, as a walk would filter it.
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

// Collect returns the inputs target refers to: a file, every file below a
// directory, the entries of an archive, or a URL. As in ripgrep, a named
// target is never filtered itself, only the files below it. Unreadable
// inputs are returned as failures; a bad URL's Err is a *URLError. The only
// error is ctx's.
func (c *Collector) Collect(ctx context.Context, target string, opts Options) ([]sources.InputFile, []core.Failure, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if archiveURL, ok := sources.RewriteRepoURL(target); ok {
		c.log.Debug("Rewrote repo URL to archive", "from", target, "to", archiveURL)
		return c.collectRepo(ctx, target, archiveURL, opts)
	}
	if sources.IsHTTPURL(target) {
		return c.collectURL(ctx, target, opts)
	}

	if opts.Piped {
		// A piped path stands for a walked path, which is always clean.
		target = filepath.Clean(target)
	}
	absPath := osenv.Abs(c.cwd, target)
	stat, err := os.Stat(absPath)
	w := c.walker(ctx, opts)
	if opts.Piped && !w.Accepts(target, err == nil && (stat.IsDir() || expands(opts, target))) {
		c.log.Debug("Skipping piped path due to filters", "path", target)
		return nil, nil, nil
	}
	if err != nil {
		// The output file is never collected, even named before it exists.
		if c.isOutput(target, opts.OutPath) {
			return nil, nil, nil
		}
		return nil, []core.Failure{{Kind: core.FailedStat, Path: absPath, Display: target, Err: err}}, nil
	}
	return c.walk(ctx, w, newRoot(c.cwd, target, absPath, stat.IsDir()), opts)
}

// expands reports whether the file at path is an archive opts expand, which
// the globs match as a directory.
func expands(opts Options, path string) bool {
	return opts.ExpandArchives && sources.IsArchivePath(path)
}

// UserIgnoreRules reads lx's user ignore file and git's global excludes,
// looking up variables through env as os.LookupEnv does. A nil log means
// slog.Default().
func UserIgnoreRules(env func(key string) (string, bool), log *slog.Logger) (ignoreRules, gitExcludes []string) {
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
// itself; an archive's entries are, as the paths below the URL.
func (c *Collector) collectURL(ctx context.Context, rawURL string, opts Options) ([]sources.InputFile, []core.Failure, error) {
	if opts.ExpandArchives && sources.IsHTTPArchiveURL(rawURL) {
		return c.downloadArchive(ctx, rawURL, archiveSource{display: rawURL, ovPrefix: strings.TrimSuffix(rawURL, "/") + "/", named: true}, opts)
	}

	urlFile, err := sources.NewURLInputFile(rawURL)
	if err != nil {
		return nil, urlFailure(&URLError{URL: rawURL, Err: err}), nil
	}
	c.log.Debug("URL accepted", "url", urlFile.Path)
	return []sources.InputFile{urlFile}, nil, nil
}

func urlFailure(err *URLError) []core.Failure {
	return []core.Failure{{Kind: core.FailedURL, Path: err.URL, Display: err.URL, Err: err}}
}

// collectRepo collects a repository URL's files from its host's archive at
// archiveURL as from a checkout: shown below repo, matched by the globs from
// the repository root, with its ignore files and git excludes applied.
func (c *Collector) collectRepo(ctx context.Context, repo, archiveURL string, opts Options) ([]sources.InputFile, []core.Failure, error) {
	return c.downloadArchive(ctx, archiveURL, archiveSource{display: strings.TrimSuffix(repo, "/"), repo: true, named: true}, opts)
}

// downloadArchive expands the archive at rawURL as src describes it.
func (c *Collector) downloadArchive(ctx context.Context, rawURL string, src archiveSource, opts Options) ([]sources.InputFile, []core.Failure, error) {
	tempPath, cleanup, err := sources.DownloadURLToTempFile(ctx, rawURL, opts.TempDir)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, ctxErr
		}
		return nil, urlFailure(&URLError{URL: rawURL, Download: true, Err: err}), nil
	}
	// Expansion reads the entries into memory.
	defer cleanup()
	src.origin = rawURL
	return c.expandArchive(ctx, tempPath, src, opts, "Ignored in URL archive")
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

func (c *Collector) walk(ctx context.Context, w *walker.Walker, r root, opts Options) ([]sources.InputFile, []core.Failure, error) {
	c.log.Debug("Walking target", "root", r.rawPath, "globs", len(opts.Globs))
	anchor := c.cache.Anchor(r.rawPath)

	var files []sources.InputFile
	var failures []core.Failure
	fail := func(path string, err error) {
		failures = append(failures, core.Failure{Kind: core.FailedWalk, Path: r.abs(path), Display: r.display(path), Err: err})
	}
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
			fail(path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}

		display := r.display(path)
		if expands(opts, path) {
			rel := "."
			if r.isDir {
				rel = path
			}
			entries, entryFailures, err := c.expandArchive(ctx, r.abs(path), archiveSource{origin: r.abs(path), display: display, ovPrefix: anchor.Path(rel) + "/", named: !r.isDir}, opts, "Ignored in archive")
			files = append(files, entries...)
			failures = append(failures, entryFailures...)
			return err
		}
		if c.isOutput(display, opts.OutPath) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			fail(path, err)
			return nil
		}
		// A symlink's own size is its target path's length; use the target's.
		if info.Mode()&fs.ModeSymlink != 0 {
			if target, statErr := fs.Stat(r.fsys, path); statErr == nil {
				info = target
			}
		}
		// Skip walked FIFOs, sockets and devices, as rg does: a FIFO with no
		// writer would block. A named one is read as a stream.
		if r.isDir && sources.IsIrregular(info.Mode()) {
			c.log.Debug("Skipped irregular file", "path", display, "mode", info.Mode().String())
			return nil
		}

		f := sources.NewInputFile(r.fsys, path, info)
		f.Path = display
		f.AbsPath = r.abs(path)

		c.log.Debug("File accepted by walker", "path", f.Path, "size", f.Size)
		files = append(files, f)
		return nil
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, nil, ctxErr
	}
	if err != nil {
		failures = append(failures, core.Failure{Kind: core.FailedWalk, Path: r.absPath, Display: r.rawPath, Err: err})
	}
	c.log.Debug("Walker finished", "root", r.walkRoot, "files_matched", len(files))
	return files, failures, nil
}

// archiveSource is an archive to expand. ovPrefix is what the globs match
// before each entry's path, named marks an archive named rather than walked,
// and repo a repository host's archive, expanded as a checkout.
type archiveSource struct {
	origin, display, ovPrefix string
	named, repo               bool
}

// expandArchive returns the entries of the archive at absPath that opts
// keep. Its only error is ctx's.
func (c *Collector) expandArchive(ctx context.Context, absPath string, src archiveSource, opts Options, ignoreMsg string) ([]sources.InputFile, []core.Failure, error) {
	displayPath := src.display
	wopts := walker.Options{
		Overrides:      opts.Globs,
		OverridePrefix: src.ovPrefix,
		SkipHidden:     !opts.ShowHidden,
		Log:            c.log,
	}
	if src.repo {
		wopts.RepoRoot = true
		wopts.FollowDirSymlinks, wopts.SkipFileSymlinks = opts.FollowDirSymlinks, opts.SkipFileSymlinks
		if !opts.NoIgnore {
			wopts.IgnoreEnabled = true
			wopts.Base, wopts.GitExcludes = opts.IgnoreRules, opts.GitExcludes
		}
	}
	if c.debugEnabled(ctx) {
		wopts.OnIgnore = func(p, reason string) {
			c.log.Debug(ignoreMsg, "path", displayPath+"/"+p, "reason", reason)
		}
	}
	aopts := sources.ArchiveOptions{
		Display: displayPath,
		Origin:  src.origin,
		Walker:  walker.New(wopts),
		MaxSize: opts.MaxSize,
		Named:   src.named,
		Log:     c.log,
	}
	expand := sources.ExpandArchive
	if src.repo {
		expand = sources.ExpandRepoArchive
	}
	files, failures, err := expand(ctx, absPath, aopts)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, nil, ctxErr
	}
	if err != nil {
		failures = append(failures, core.Failure{Kind: core.FailedArchive, Path: src.origin, Display: displayPath, Err: err})
	}
	return files, failures, nil
}

// isOutput reports whether path, relative to the working directory, is
// outPath.
func (c *Collector) isOutput(path, outPath string) bool {
	if outPath == "" {
		return false
	}
	if osenv.Abs(c.cwd, path) == outPath {
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
