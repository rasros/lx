package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mholt/archives"
	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/walker"
)

// Multi-part suffixes must come before shorter suffixes.
var archiveSuffixes = []string{
	".zip", ".jar", ".war", ".ear", ".odt", ".ods", ".odp",
	".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst",
	".tar.br", ".tar.lz4", ".tar.sz", ".tar.s2",
	".tgz", ".tbz2", ".txz",
	".tar",
	".rar", ".7z",
	".gz", ".bz2", ".xz", ".zst", ".br", ".lz4", ".sz", ".s2", ".lz",
}

func IsArchivePath(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range archiveSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// ArchiveOptions say how ExpandArchive names and picks the entries of an
// archive.
type ArchiveOptions struct {
	// Display is the path the entries are displayed below.
	Display string
	// Origin names the archive in each entry's InputFile.Archive: its
	// absolute path, or its URL when it was downloaded.
	Origin string
	// Walker picks the entries, as its Walk would pick them in an fs.FS of
	// the archive; nil keeps them all.
	Walker *walker.Walker
	// MaxSize, when positive, is the size over which an entry is not read.
	// Such an entry is returned with its size alone, for the stream to drop
	// as it drops any file over its RenderOptions.MaxSize, which MaxSize
	// should be.
	MaxSize int64
	// Log is told about entries that cannot be read, and skipped ones;
	// nil logs to slog.Default().
	Log *slog.Logger
	// Named marks an archive that was named, as a path or URL, rather than
	// found in a walk. The one entry of a named compressed file is the named
	// file's content, so, as a named file, it is not skipped as hidden.
	Named bool
}

// ExpandArchive returns the files in the archive at absPath that opts.Walker
// keeps, in the order a walk would find them. The archive is read once, from
// start to end, and only the entries kept and no larger than opts.MaxSize
// are read, into memory, so the archive is not needed once it returns. An
// entry whose path leaves the archive, through "..", is skipped. An archive
// that cannot be read to its end fails with the entries read before it
// failed. The entries that cannot be read, or end before the size the
// archive claims for them, are returned as failures. An archive holding a file and a directory of the same name, as
// no filesystem can, has both returned, but a tree of them shows only the
// directory.
func ExpandArchive(ctx context.Context, absPath string, opts ArchiveOptions) ([]InputFile, []core.Failure, error) {
	if !IsArchivePath(absPath) {
		return nil, nil, nil
	}
	if opts.Walker == nil {
		opts.Walker = walker.New(walker.Options{})
	}
	file, err := os.Open(absPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	format, err := identifyArchive(ctx, file, filepath.Base(absPath))
	if err != nil {
		return nil, nil, err
	}

	x := expansion{opts: opts, log: opts.Log, base: archiveBase(opts.Display), entries: opts.Walker.Entries(), seen: map[string]bool{}}
	if x.log == nil {
		x.log = slog.Default()
	}
	switch f := format.(type) {
	case archives.Extractor:
		err = f.Extract(ctx, file, x.handle)
	case archives.Decompressor:
		err = x.decompress(f, file)
	default:
		return nil, nil, errors.New("no format lx can read")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, nil, ctxErr
	}
	if n := len(x.failures); err != nil && n > 0 && errors.Is(err, x.failures[n-1].Err) {
		// The entry that failed ended the archive, as a tar entry cut short
		// does: one failure, counted once.
		err = nil
	}
	if err != nil {
		err = fmt.Errorf("extract: %w", err)
	}

	x.sort()
	files := make([]InputFile, len(x.files))
	for i, e := range x.files {
		files[i] = e.file
	}
	x.log.Debug("Archive expansion finished", "archive", opts.Display, "files_found", len(files), "failures", len(x.failures))
	return files, x.failures, err
}

// identifyArchive identifies the format of the archive file, named name,
// by its content, as a .zip named .tar.gz is a zip archive, or by its name
// when the content does not say, as for a brotli stream, which has no
// header. It leaves file at its start. Its error is the same for every
// file no format fits.
func identifyArchive(ctx context.Context, file *os.File, name string) (archives.Format, error) {
	format, _, err := archives.Identify(ctx, "", file)
	if errors.Is(err, archives.NoMatch) {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		format, _, err = archives.Identify(ctx, name, file)
	}
	if err != nil {
		// Identify tries the formats in no fixed order and names the one
		// it failed on, so the error says no more than that none fit.
		return nil, errors.New("no format lx can read")
	}
	// Identify reads the head of a seekable stream and rewinds it, but the
	// extractors read from where the file is.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return format, nil
}

// archiveBase is the path the entries of an archive displayed as display
// are below.
func archiveBase(display string) string {
	if IsHTTPURL(display) {
		return strings.TrimSuffix(display, "/")
	}
	return filepath.ToSlash(filepath.Clean(display))
}

// expansion collects the entries of one archive as it is read.
type expansion struct {
	opts    ArchiveOptions
	log     *slog.Logger
	base    string
	entries *walker.Entries
	seen    map[string]bool
	files   []entryFile
	// failures are the entries that could not be read.
	failures []core.Failure
}

// fail records that the entry at key, which is entry in the archive, could
// not be read. It is named below the archive's origin, as the inputs it
// would have been are keyed.
func (x *expansion) fail(key, entry string, err error) {
	x.failures = append(x.failures, core.Failure{
		Kind:    core.FailedArchiveEntry,
		Path:    x.opts.Origin + "/" + entry,
		Display: x.base + "/" + key,
		Err:     err,
	})
}

type entryFile struct {
	key  string
	file InputFile
}

// handle decides on one entry of an archive being extracted: a directory
// the walker skips is skipped whole, and a file is read only when it is
// kept and no larger than MaxSize.
func (x *expansion) handle(ctx context.Context, info archives.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key := normalizeArchiveEntryPath(info.NameInArchive)
	if key == "" {
		return nil
	}
	if key == ".." || strings.HasPrefix(key, "../") {
		x.log.Debug("Skipping archive entry outside the archive", "archive", x.opts.Display, "path", info.NameInArchive)
		return nil
	}
	if info.IsDir() {
		if !x.entries.Keep(key, true) {
			return fs.SkipDir
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		x.log.Debug("Skipping archive entry that is not a regular file", "archive", x.opts.Display, "path", key, "mode", info.Mode().String())
		return nil
	}
	if !x.entries.Keep(key, false) {
		return nil
	}
	if x.seen[key] {
		x.log.Warn("Skipping duplicate archive entry path", "archive", x.opts.Display, "path", key)
		return nil
	}
	x.seen[key] = true

	if x.over(info.Size()) {
		x.add(key, key, info.Size(), info, nil)
		return nil
	}
	rc, err := info.Open()
	if err != nil {
		x.fail(key, key, err)
		return nil
	}
	data, n, err := readEntry(rc, info.Size(), x.opts.MaxSize)
	_ = rc.Close()
	if err != nil {
		x.fail(key, key, err)
		return nil
	}
	x.add(key, key, n, info, data)
	return nil
}

// decompress reads the one entry of a compressed file, named after it
// without the compression suffix, so the entry of notes.txt.gz is
// notes.txt.gz/notes.txt.
func (x *expansion) decompress(d archives.Decompressor, file *os.File) error {
	name := singleEntryName(x.opts.Origin)
	keep := x.entries.Keep
	if x.opts.Named {
		keep = x.entries.KeepShown
	}
	if !keep(name, false) {
		return nil
	}
	rc, err := d.OpenReader(file)
	if err != nil {
		return err
	}
	defer rc.Close()
	// The size of the decompressed content is known only once it is read.
	// Unlike a Stream it is not capped at DefaultStreamLimit without -m: it
	// ends, as the entries of any archive do, which are not capped either.
	data, n, err := readEntry(rc, -1, x.opts.MaxSize)
	if err != nil {
		name := singleEntryName(x.opts.Origin)
		x.fail(name, name, err)
		return nil
	}
	var info fs.FileInfo
	if st, err := file.Stat(); err == nil {
		info = st
	}
	x.add(name, name, n, info, data)
	return nil
}

// compressionSuffixes are the suffixes of single-file compression, and what
// each leaves of a name: ".tgz" is a compressed ".tar", which is a plain
// file when the content is not a tar archive.
var compressionSuffixes = [][2]string{
	{".tgz", ".tar"}, {".tbz2", ".tar"}, {".txz", ".tar"},
	{".gz", ""}, {".bz2", ""}, {".xz", ""}, {".zst", ""}, {".br", ""},
	{".lz4", ""}, {".sz", ""}, {".s2", ""}, {".lz", ""},
}

// singleEntryName names the content of the compressed file at origin, a
// path or URL: its name without the compression suffix, so foo.tar.gz that
// is no tar archive holds foo.tar, or its name when that is all there is.
func singleEntryName(origin string) string {
	name := filepath.Base(origin)
	if u, err := parseHTTPURL(origin); err == nil {
		name = path.Base(u.Path)
	}
	lower := strings.ToLower(name)
	for _, s := range compressionSuffixes {
		if strings.HasSuffix(lower, s[0]) && len(name) > len(s[0]) {
			stem, suffix := name[:len(name)-len(s[0])], s[1]
			if written := name[len(stem):]; written == strings.ToUpper(written) {
				// NOTAR.TGZ holds NOTAR.TAR, in the case it was named in.
				suffix = strings.ToUpper(suffix)
			}
			return stem + suffix
		}
	}
	return name
}

// over reports whether an entry of size bytes is too large to read.
func (x *expansion) over(size int64) bool {
	return x.opts.MaxSize > 0 && size > x.opts.MaxSize
}

// add adds the entry displayed at key, which is entry in the archive, of
// size bytes. Its content is data, or, when it is over MaxSize, unread.
func (x *expansion) add(key, entry string, size int64, info fs.FileInfo, data *content) {
	f := InputFile{
		Path:    x.base + "/" + key,
		Archive: x.opts.Origin,
		Entry:   entry,
		Size:    size,
	}
	if info != nil {
		f.ModTime = info.ModTime()
	}
	if x.over(size) {
		x.log.Debug("Archive entry over the size limit left unread", "archive", x.opts.Display, "path", key, "size", size)
		f.Open = func(context.Context) (io.ReadCloser, error) {
			return nil, fmt.Errorf("%s is over the size limit and was not read", f.Path)
		}
	} else {
		f.Open = func(context.Context) (io.ReadCloser, error) { return data.open(), nil }
	}
	x.files = append(x.files, entryFile{key: key, file: f})
}

// sort puts the entries in the order a walk finds them: depth first, each
// directory's entries by name.
func (x *expansion) sort() {
	slices.SortStableFunc(x.files, func(a, b entryFile) int {
		return compareWalkOrder(a.key, b.key)
	})
}

// compareWalkOrder orders slash-separated paths as a walk visits them,
// comparing name by name, so "a/x" comes before "a.txt".
func compareWalkOrder(a, b string) int {
	for {
		an, arest, amore := strings.Cut(a, "/")
		bn, brest, bmore := strings.Cut(b, "/")
		if c := strings.Compare(an, bn); c != 0 {
			return c
		}
		switch {
		case !amore && !bmore:
			return 0
		case !amore:
			return -1
		case !bmore:
			return 1
		}
		a, b = arest, brest
	}
}

func normalizeArchiveEntryPath(p string) string {
	if p == "" {
		return ""
	}
	cleaned := path.Clean(filepath.ToSlash(p))
	cleaned = strings.TrimPrefix(cleaned, "./")
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}
