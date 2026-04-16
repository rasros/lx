package sources

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
	"github.com/rasros/lx/pkg/lx/walker"
)

// multi-part extensions must appear before their shorter suffix (.tar.gz before .gz).
var archiveSuffixes = []string{
	// ZIP-based (including document formats that are ZIP archives)
	".zip", ".jar", ".war", ".ear", ".odt", ".ods", ".odp",
	// TAR with compression (multi-part first)
	".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst",
	".tar.br", ".tar.lz4", ".tar.sz", ".tar.s2",
	".tgz", ".tbz2", ".txz",
	// Plain TAR
	".tar",
	// Other multi-file archives
	".rar", ".7z",
	// Single-file compression (library exposes one virtual entry)
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

// FileSink receives expanded archive files.
type FileSink interface {
	Add(InputFile)
}

// ExpandArchivePaths opens the archive at absPath and returns the display paths
// of all matching entries without reading their content.
func ExpandArchivePaths(ctx context.Context, absPath, displayPath string, w *walker.Walker, includes []string) ([]string, error) {
	var paths []string
	sink := &pathCollectorSink{out: &paths}
	if err := ExpandArchive(ctx, absPath, displayPath, w, includes, "", sink); err != nil {
		return nil, err
	}
	return paths, nil
}

type pathCollectorSink struct{ out *[]string }

func (s *pathCollectorSink) Add(f InputFile) { *s.out = append(*s.out, f.Path) }

// ExpandArchive opens the archive at absPath and emits each file entry.
func ExpandArchive(ctx context.Context, absPath, displayPath string, w *walker.Walker, includes []string, outPath string, sink FileSink) error {
	if !IsArchivePath(absPath) {
		return nil
	}
	fsys, err := archives.FileSystem(ctx, absPath, nil)
	if err != nil {
		return fmt.Errorf("open archive %q: %w", absPath, err)
	}

	archiveBase := filepath.ToSlash(filepath.Clean(displayPath))
	if IsHTTPURL(displayPath) {
		archiveBase = strings.TrimSuffix(displayPath, "/")
	}
	count := 0

	err = w.Walk(fsys, ".", func(entryPath string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("Error accessing archive entry", "archive", displayPath, "path", entryPath, "error", err)
			return nil
		}
		if d.IsDir() {
			return nil
		}

		effectivePath := archiveBase + "/" + entryPath

		if len(includes) > 0 {
			matched := false
			for _, inc := range includes {
				if walker.IsMatch(inc, entryPath) {
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

		capturedEntry := entryPath
		capturedAbs := absPath
		capturedCtx := ctx

		f := InputFile{
			Path:    effectivePath,
			AbsPath: absPath + "/" + entryPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Open: func() (io.ReadCloser, error) {
				af, err := archives.FileSystem(capturedCtx, capturedAbs, nil)
				if err != nil {
					return nil, fmt.Errorf("reopen archive %q: %w", capturedAbs, err)
				}
				file, err := af.Open(capturedEntry)
				if err != nil {
					closeFS(af)
					return nil, err
				}
				return &archiveEntryReader{File: file, fsCloser: af}, nil
			},
		}

		slog.Debug("File accepted from archive", "path", f.Path, "size", f.Size)
		sink.Add(f)
		count++
		return nil
	})

	slog.Debug("Archive expansion finished", "archive", displayPath, "files_found", count)
	return err
}

func closeFS(fsys fs.FS) {
	if c, ok := fsys.(io.Closer); ok {
		_ = c.Close()
	}
}

type archiveEntryReader struct {
	fs.File
	fsCloser fs.FS
}

func (r *archiveEntryReader) Close() error {
	err := r.File.Close()
	closeFS(r.fsCloser)
	return err
}
