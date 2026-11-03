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
	"strings"
	"time"

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

type archiveCandidate struct {
	key           string
	entryPath     string
	effectivePath string
	size          int64
	modTime       time.Time
	data          []byte
	materialized  bool
}

// ExpandArchive returns the files in the archive at absPath, displayed below
// displayPath, that w walks to, and the entries that could not be read.
// Entries are read into memory, so the archive is not needed once it returns.
// It logs to log, or slog.Default() when nil.
func ExpandArchive(ctx context.Context, absPath, displayPath string, w *walker.Walker, log *slog.Logger) ([]InputFile, []core.Failure, error) {
	if log == nil {
		log = slog.Default()
	}
	if !IsArchivePath(absPath) {
		return nil, nil, nil
	}
	fsys, err := archives.FileSystem(ctx, absPath, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("open archive %q: %w", absPath, err)
	}
	defer closeFS(fsys)

	archiveBase := filepath.ToSlash(filepath.Clean(displayPath))
	if IsHTTPURL(displayPath) {
		archiveBase = strings.TrimSuffix(displayPath, "/")
	}

	var candidates []archiveCandidate
	var failures []core.Failure
	// A failure is named below the archive's absolute path, or its URL.
	key := absPath
	if IsHTTPURL(displayPath) {
		key = archiveBase
	}
	fail := func(entryPath string, err error) {
		f := core.Failure{Kind: core.FailedArchive, Path: key, Display: archiveBase, Err: err}
		if entryPath != "." {
			f.Kind = core.FailedArchiveEntry
			f.Path += "/" + entryPath
			f.Display += "/" + entryPath
		}
		failures = append(failures, f)
	}
	candidateByKey := make(map[string]int)

	err = w.Walk(fsys, ".", func(entryPath string, d fs.DirEntry, err error) error {
		if err != nil {
			fail(entryPath, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		candidatePath := entryPath
		if candidatePath == "." {
			candidatePath = d.Name()
		}

		key := normalizeArchiveEntryPath(candidatePath)
		if key == "" {
			return nil
		}
		effectivePath := archiveBase + "/" + key

		info, err := d.Info()
		if err != nil {
			fail(entryPath, err)
			return nil
		}

		if _, exists := candidateByKey[key]; exists {
			log.Warn("Skipping duplicate archive entry path", "archive", displayPath, "path", key)
			return nil
		}

		candidateByKey[key] = len(candidates)
		candidates = append(candidates, archiveCandidate{
			key:           key,
			entryPath:     candidatePath,
			effectivePath: effectivePath,
			size:          info.Size(),
			modTime:       info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	if len(candidates) == 0 {
		log.Debug("Archive expansion finished", "archive", displayPath, "files_found", 0)
		return nil, failures, nil
	}

	usedExtractor, err := materializeWithExtractor(ctx, absPath, candidates, candidateByKey, log)
	if err != nil {
		return nil, nil, err
	}

	for i := range candidates {
		if candidates[i].materialized {
			continue
		}
		data, err := snapshotArchiveEntry(fsys, candidates[i].entryPath, candidates[i].size)
		if err != nil {
			fail(candidates[i].entryPath, err)
			continue
		}
		candidates[i].data = data
		candidates[i].materialized = true
	}

	var files []InputFile
	for i := range candidates {
		if !candidates[i].materialized {
			continue
		}
		cached := candidates[i].data
		f := InputFile{
			Path:    candidates[i].effectivePath,
			AbsPath: absPath + "/" + candidates[i].entryPath,
			Size:    int64(len(cached)),
			ModTime: candidates[i].modTime,
			Open: func() (io.ReadCloser, error) {
				return newByteReaderReadCloser(cached), nil
			},
		}
		log.Debug("File accepted from archive", "path", f.Path, "size", f.Size)
		files = append(files, f)
	}

	log.Debug("Archive expansion finished", "archive", displayPath, "files_found", len(files), "used_extractor", usedExtractor)
	return files, failures, nil
}

func closeFS(fsys fs.FS) {
	if c, ok := fsys.(io.Closer); ok {
		_ = c.Close()
	}
}

const maxInt = int64(^uint(0) >> 1)

func readAllWithSizeHint(r io.Reader, sizeHint int64) ([]byte, error) {
	if sizeHint <= 0 || sizeHint > maxInt {
		return io.ReadAll(r)
	}

	// A short entry is one that ends cleanly; io.ErrUnexpectedEOF from the
	// reader is a corrupt one, as a cut-off compressed stream is.
	buf := make([]byte, int(sizeHint))
	n := 0
	for n < len(buf) {
		m, err := r.Read(buf[n:])
		n += m
		if errors.Is(err, io.EOF) {
			return buf[:n], nil
		}
		if err != nil {
			return nil, err
		}
	}

	// Size hints can be inaccurate for some formats. Keep reading if there is tail data.
	var probe [1]byte
	m, readErr := r.Read(probe[:])
	if errors.Is(readErr, io.EOF) {
		return buf, nil
	}
	if readErr != nil {
		return nil, readErr
	}

	tail, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, n+m+len(tail))
	out = append(out, buf...)
	out = append(out, probe[:m]...)
	out = append(out, tail...)
	return out, nil
}

func snapshotArchiveEntry(fsys fs.FS, path string, sizeHint int64) ([]byte, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if sizeHint <= 0 {
		if info, statErr := f.Stat(); statErr == nil {
			sizeHint = info.Size()
		}
	}
	return readAllWithSizeHint(f, sizeHint)
}

func materializeWithExtractor(ctx context.Context, absPath string, candidates []archiveCandidate, candidateByKey map[string]int, log *slog.Logger) (bool, error) {
	file, err := os.Open(absPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	format, _, err := archives.Identify(ctx, filepath.Base(absPath), file)
	if err != nil {
		if errors.Is(err, archives.NoMatch) {
			return false, nil
		}
		return false, fmt.Errorf("identify archive %q: %w", absPath, err)
	}

	extractor, ok := format.(archives.Extractor)
	if !ok {
		return false, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return true, fmt.Errorf("rewind archive %q: %w", absPath, err)
	}

	err = extractor.Extract(ctx, file, func(ctx context.Context, info archives.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		key := normalizeArchiveEntryPath(info.NameInArchive)
		idx, ok := candidateByKey[key]
		if !ok || candidates[idx].materialized {
			return nil
		}

		rc, err := info.Open()
		if err != nil {
			// The entry is read again below; a failure there is reported.
			log.Debug("Failed to open extractor entry", "archive", absPath, "path", key, "error", err)
			return nil
		}
		data, readErr := readAllWithSizeHint(rc, info.Size())
		closeErr := rc.Close()
		if readErr != nil {
			log.Debug("Failed to read extractor entry", "archive", absPath, "path", key, "error", readErr)
			return nil
		}
		if closeErr != nil {
			log.Debug("Failed to close extractor entry", "archive", absPath, "path", key, "error", closeErr)
			return nil
		}

		candidates[idx].data = data
		candidates[idx].materialized = true
		if candidates[idx].modTime.IsZero() {
			candidates[idx].modTime = info.ModTime()
		}
		if candidates[idx].size <= 0 {
			candidates[idx].size = int64(len(data))
		}
		return nil
	})
	if err != nil {
		return true, fmt.Errorf("extract archive %q: %w", absPath, err)
	}

	return true, nil
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
