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

type FileSink interface {
	Add(InputFile)
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

func ExpandArchive(ctx context.Context, absPath, displayPath string, w *walker.Walker, includes []string, outPath string, sink FileSink) error {
	if !IsArchivePath(absPath) {
		return nil
	}
	fsys, err := archives.FileSystem(ctx, absPath, nil)
	if err != nil {
		return fmt.Errorf("open archive %q: %w", absPath, err)
	}
	defer closeFS(fsys)

	archiveBase := filepath.ToSlash(filepath.Clean(displayPath))
	if IsHTTPURL(displayPath) {
		archiveBase = strings.TrimSuffix(displayPath, "/")
	}

	var candidates []archiveCandidate
	candidateByKey := make(map[string]int)

	err = w.Walk(fsys, ".", func(entryPath string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("Error accessing archive entry", "archive", displayPath, "path", entryPath, "error", err)
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

		if len(includes) > 0 {
			matched := false
			for _, inc := range includes {
				if walker.IsMatch(inc, key) {
					matched = true
					break
				}
			}
			if !matched {
				slog.Debug("Ignored by include filter (-i)", "path", effectivePath)
				return nil
			}
		}

		if outPath != "" {
			if abs, _ := filepath.Abs(effectivePath); abs == outPath {
				slog.Warn("Skipping output file to avoid infinite recursion", "path", effectivePath)
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			slog.Error("Failed to stat archive entry", "path", entryPath, "error", err)
			return nil
		}

		if _, exists := candidateByKey[key]; exists {
			slog.Warn("Skipping duplicate archive entry path", "archive", displayPath, "path", key)
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
		return err
	}

	if len(candidates) == 0 {
		slog.Debug("Archive expansion finished", "archive", displayPath, "files_found", 0)
		return nil
	}

	usedExtractor, err := materializeWithExtractor(ctx, absPath, candidates, candidateByKey)
	if err != nil {
		return err
	}

	for i := range candidates {
		if candidates[i].materialized {
			continue
		}
		data, err := snapshotArchiveEntry(fsys, candidates[i].entryPath, candidates[i].size)
		if err != nil {
			slog.Warn("Failed to read archive entry", "archive", displayPath, "path", candidates[i].entryPath, "error", err)
			continue
		}
		candidates[i].data = data
		candidates[i].materialized = true
	}

	count := 0
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
		slog.Debug("File accepted from archive", "path", f.Path, "size", f.Size)
		sink.Add(f)
		count++
	}

	slog.Debug("Archive expansion finished", "archive", displayPath, "files_found", count, "used_extractor", usedExtractor)
	return nil
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

	buf := make([]byte, int(sizeHint))
	n, err := io.ReadFull(r, buf)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return buf[:n], nil
		}
		return nil, err
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

func materializeWithExtractor(ctx context.Context, absPath string, candidates []archiveCandidate, candidateByKey map[string]int) (bool, error) {
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
			slog.Warn("Failed to open extractor entry", "archive", absPath, "path", key, "error", err)
			return nil
		}
		data, readErr := readAllWithSizeHint(rc, info.Size())
		closeErr := rc.Close()
		if readErr != nil {
			slog.Warn("Failed to read extractor entry", "archive", absPath, "path", key, "error", readErr)
			return nil
		}
		if closeErr != nil {
			slog.Warn("Failed to close extractor entry", "archive", absPath, "path", key, "error", closeErr)
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
