package sources

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/walker"
)

// ExpandRepoArchive returns the files of the repository archived in the zip
// at absPath, as a repository host serves one, that opts.Walker, or a
// walker keeping every file when nil, keeps when it walks the archive as it
// would a checkout. Hosts wrap the files in one directory, such as
// "repo-main/", which is left out of their displayed and matched paths but
// kept in InputFile.Entry; an archive with anything but one directory at
// the top is walked from its root. A walker with walker.Options.RepoRoot
// and IgnoreEnabled set applies the repository's ignore files as a walk of
// a checkout does.
//
// The zip is walked as the fs.FS archive/zip makes of it. Symbolic links are
// skipped. Each file kept and no larger than opts.MaxSize is read once, into
// memory, so the archive is not needed once it returns.
func ExpandRepoArchive(ctx context.Context, absPath string, opts ArchiveOptions) ([]InputFile, []core.Failure, error) {
	if opts.Walker == nil {
		opts.Walker = walker.New(walker.Options{})
	}
	zr, err := zip.OpenReader(absPath)
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && zr != nil) {
		return nil, nil, fmt.Errorf("open: %w", err)
	}
	defer zr.Close()

	x := expansion{opts: opts, log: opts.Log, base: archiveBase(opts.Display)}
	if x.log == nil {
		x.log = slog.Default()
	}
	var fsys fs.FS = &zr.Reader
	entryPrefix := ""
	if wrapper := zipWrapper(zr.File); wrapper != "" {
		if fsys, err = fs.Sub(fsys, wrapper); err != nil {
			return nil, nil, err
		}
		entryPrefix = wrapper + "/"
	}

	err = opts.Walker.Walk(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		entry := entryPrefix + p
		if err != nil {
			x.fail(p, entry, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			x.log.Debug("Skipping archive entry that is not a regular file", "archive", opts.Display, "path", p)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			x.fail(p, entry, err)
			return nil
		}
		if x.over(info.Size()) {
			x.add(p, entry, info.Size(), info, nil)
			return nil
		}
		f, err := fsys.Open(p)
		if err != nil {
			x.fail(p, entry, err)
			return nil
		}
		data, n, err := readEntry(f, info.Size(), opts.MaxSize)
		_ = f.Close()
		if err != nil {
			x.fail(p, entry, err)
			return nil
		}
		x.add(p, entry, n, info, data)
		return nil
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, nil, ctxErr
	}
	if err != nil {
		err = fmt.Errorf("walk: %w", err)
	}
	files := make([]InputFile, len(x.files))
	for i, e := range x.files {
		files[i] = e.file
	}
	x.log.Debug("Repository archive expansion finished", "archive", opts.Display, "wrapper", entryPrefix, "files_found", len(files), "failures", len(x.failures))
	return files, x.failures, err
}

// zipWrapper returns the one directory every entry of a zip is below: the
// first entry's top directory, or "" when some entry is not below it.
func zipWrapper(files []*zip.File) string {
	if len(files) == 0 {
		return ""
	}
	top, _, nested := strings.Cut(files[0].Name, "/")
	if !nested {
		return ""
	}
	for _, f := range files {
		if f.Name != top+"/" && !strings.HasPrefix(f.Name, top+"/") {
			return ""
		}
	}
	return top
}
