package lx

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
)

var zipExtensions = map[string]bool{
	".zip": true,
	".jar": true,
	".war": true,
	".ear": true,
}

// IsArchivePath reports whether path has a recognised archive extension.
func IsArchivePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return zipExtensions[ext]
}

// ExpandArchive opens the archive at absPath and walks its contents using walker,
// adding matched entries to stream. displayPath is prepended to entry paths.
// includes is a post-walk filter applied to entry paths (empty means all pass).
// outPath, if non-empty, is skipped to avoid infinite recursion.
func ExpandArchive(absPath, displayPath string, walker *Walker, includes []string, outPath string, stream *Stream) error {
	ext := strings.ToLower(filepath.Ext(absPath))
	if zipExtensions[ext] {
		return expandZip(absPath, displayPath, walker, includes, outPath, stream)
	}
	return nil
}

func expandZip(absPath, displayPath string, walker *Walker, includes []string, outPath string, stream *Stream) error {
	r, err := zip.OpenReader(absPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	archiveBase := filepath.ToSlash(filepath.Clean(displayPath))
	count := 0

	err = walker.Walk(&r.Reader, ".", func(entryPath string, d fs.DirEntry, err error) error {
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
				if IsMatch(inc, entryPath) {
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

		f := InputFile{
			Path:    effectivePath,
			AbsPath: absPath + "/" + entryPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Open: func() (io.ReadCloser, error) {
				ar, err := zip.OpenReader(capturedAbs)
				if err != nil {
					return nil, fmt.Errorf("reopen zip: %w", err)
				}
				for _, ze := range ar.File {
					if ze.Name == capturedEntry {
						rc, err := ze.Open()
						if err != nil {
							ar.Close()
							return nil, err
						}
						return &zipEntryReader{rc: rc, ar: ar}, nil
					}
				}
				ar.Close()
				return nil, fmt.Errorf("entry %q not found in zip", capturedEntry)
			},
		}

		slog.Debug("File accepted from archive", "path", f.Path, "size", f.Size)
		stream.AddFile(f)
		count++
		return nil
	})

	slog.Debug("Archive expansion finished", "archive", displayPath, "files_found", count)
	return err
}

// zipEntryReader wraps a zip entry reader and closes the parent archive on Close.
type zipEntryReader struct {
	rc io.ReadCloser
	ar io.Closer
}

func (z *zipEntryReader) Read(p []byte) (int, error) { return z.rc.Read(p) }

func (z *zipEntryReader) Close() error {
	err := z.rc.Close()
	z.ar.Close()
	return err
}
