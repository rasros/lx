package sources

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"slices"
	"strings"
	"time"

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
// a checkout does, since the zip can be read in any order.
//
// Entries keep the names they are stored under, backslashes included, as
// a checkout would name its files. An entry whose name has an empty, "."
// or ".." element is no file of a checkout and is skipped, as is a file
// where a directory of the same name is, and a second file of one name.
// Symbolic links are skipped too, which a checkout would list. Each file
// kept and no larger than opts.MaxSize is read once, into memory, so the
// archive is not needed once it returns.
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
	var kept []*zip.File
	for _, f := range zr.File {
		if !checkoutName(strings.TrimSuffix(f.Name, "/")) {
			x.log.Debug("Skipping repository archive entry with no checkout path", "archive", opts.Display, "path", f.Name)
			continue
		}
		kept = append(kept, f)
	}
	names := make([]string, len(kept))
	for i, f := range kept {
		names[i] = f.Name
	}
	wrapper := zipWrapper(names)
	tree := newZipTree()
	for _, f := range kept {
		name := strings.TrimSuffix(f.Name, "/")
		if wrapper != "" {
			if name == wrapper {
				continue
			}
			name = strings.TrimPrefix(name, wrapper+"/")
		}
		if reason := tree.add(name, f); reason != "" {
			x.log.Debug("Skipping repository archive entry", "archive", opts.Display, "path", f.Name, "reason", reason)
		}
	}

	err = opts.Walker.Walk(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			x.fail(p, p, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		f := tree.files[p]
		if f == nil || !f.Mode().IsRegular() {
			x.log.Debug("Skipping archive entry that is not a regular file", "archive", opts.Display, "path", p)
			return nil
		}
		info := f.FileInfo()
		if x.over(info.Size()) {
			x.add(p, f.Name, info.Size(), info, nil)
			return nil
		}
		rc, err := f.Open()
		if err != nil {
			x.fail(p, f.Name, err)
			return nil
		}
		data, n, err := readEntry(rc, info.Size(), opts.MaxSize)
		_ = rc.Close()
		if err != nil {
			x.fail(p, f.Name, err)
			return nil
		}
		x.add(p, f.Name, n, info, data)
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
	x.log.Debug("Repository archive expansion finished", "archive", opts.Display, "wrapper", wrapper, "files_found", len(files), "failures", len(x.failures))
	return files, x.failures, err
}

// checkoutName reports whether name, an entry's name without a trailing
// slash, could be the path of a file in a checkout: relative, with no
// empty, "." or ".." element and no NUL, which no filesystem allows.
func checkoutName(name string) bool {
	if name == "" || strings.ContainsRune(name, 0) {
		return false
	}
	for elem := range strings.SplitSeq(name, "/") {
		if elem == "" || elem == "." || elem == ".." {
			return false
		}
	}
	return true
}

// zipWrapper returns the one directory every entry of a zip, by its name,
// with a trailing slash for a directory, is below: the first entry's top
// directory, or "" when some entry is not below it.
func zipWrapper(names []string) string {
	if len(names) == 0 {
		return ""
	}
	top, _, nested := strings.Cut(names[0], "/")
	if !nested {
		return ""
	}
	for _, name := range names {
		if name != top+"/" && !strings.HasPrefix(name, top+"/") {
			return ""
		}
	}
	return top
}

// zipTree is the tree of a zip's entries as a checkout lays them out: an
// fs.FS of its directories, which entries name or imply, and its files.
type zipTree struct {
	files map[string]*zip.File
	// dirs holds each directory's entries, and made the directories listed
	// in the one above them.
	dirs map[string][]fs.DirEntry
	made map[string]bool
}

func newZipTree() *zipTree {
	return &zipTree{files: map[string]*zip.File{}, dirs: map[string][]fs.DirEntry{".": nil}, made: map[string]bool{".": true}}
}

// add adds the entry f at name, with the directories above it, or reports
// why it cannot be added.
func (t *zipTree) add(name string, f *zip.File) string {
	if t.files[name] != nil || (t.made[name] && !f.FileInfo().IsDir()) {
		return "a file or directory of the same name comes first"
	}
	for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
		if t.files[dir] != nil {
			return "below a file"
		}
	}
	if f.FileInfo().IsDir() {
		t.mkdir(name)
		return ""
	}
	t.mkdir(path.Dir(name))
	t.files[name] = f
	dir := path.Dir(name)
	t.dirs[dir] = append(t.dirs[dir], fs.FileInfoToDirEntry(namedInfo{f.FileInfo(), path.Base(name)}))
	return ""
}

// mkdir adds the directory at dir and those above it.
func (t *zipTree) mkdir(dir string) {
	for !t.made[dir] {
		t.made[dir] = true
		if _, ok := t.dirs[dir]; !ok {
			t.dirs[dir] = []fs.DirEntry{}
		}
		parent := path.Dir(dir)
		t.dirs[parent] = append(t.dirs[parent], fs.FileInfoToDirEntry(dirInfo(path.Base(dir))))
		dir = parent
	}
}

func (t *zipTree) Open(name string) (fs.File, error) {
	if f := t.files[name]; f != nil {
		rc, err := f.Open()
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return zipTreeFile{ReadCloser: rc, info: namedInfo{f.FileInfo(), path.Base(name)}}, nil
	}
	if _, ok := t.dirs[name]; ok {
		return zipTreeDir{name: name}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (t *zipTree) Stat(name string) (fs.FileInfo, error) {
	if f := t.files[name]; f != nil {
		return namedInfo{f.FileInfo(), path.Base(name)}, nil
	}
	if _, ok := t.dirs[name]; ok {
		return dirInfo(path.Base(name)), nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (t *zipTree) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, ok := t.dirs[name]
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	entries = slices.Clone(entries)
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// ReadFile reads the ignore files a walk looks for, which are small; one
// over maxIgnoreFile is read no further. Only regular files are read.
func (t *zipTree) ReadFile(name string) ([]byte, error) {
	f := t.files[name]
	// An ignore file stored as a link has no patterns of its own; a
	// checkout would follow it, which an archive cannot.
	if f == nil || !f.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxIgnoreFile))
}

const maxIgnoreFile = 1 << 20

type zipTreeFile struct {
	io.ReadCloser
	info fs.FileInfo
}

func (f zipTreeFile) Stat() (fs.FileInfo, error) { return f.info, nil }

type zipTreeDir struct{ name string }

func (d zipTreeDir) Stat() (fs.FileInfo, error) { return dirInfo(path.Base(d.name)), nil }
func (d zipTreeDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}
func (d zipTreeDir) Close() error { return nil }

// namedInfo is an entry's FileInfo under the name its path in the tree
// ends in.
type namedInfo struct {
	fs.FileInfo
	name string
}

func (i namedInfo) Name() string { return i.name }

// dirInfo describes a directory of the tree, which may have no entry of
// its own.
type dirInfo string

func (d dirInfo) Name() string     { return string(d) }
func (dirInfo) Size() int64        { return 0 }
func (dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o755 }
func (dirInfo) ModTime() time.Time { return time.Time{} }
func (dirInfo) IsDir() bool        { return true }
func (dirInfo) Sys() any           { return nil }
