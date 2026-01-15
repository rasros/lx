package lx

import (
	"bytes"
	"io"
	"io/fs"
	"time"
)

// InputFile represents a file to be processed.
// It holds metadata in memory (cheap) and a lazy opener for content (heavy).
type InputFile struct {
	Path      string
	AbsPath   string
	Size      int64
	ModTime   time.Time
	LoadError error

	// Config defines how this specific file should be sliced and rendered.
	Config RunnerConfig

	// Open returns a ReadCloser for the file content.
	Open func() (io.ReadCloser, error)
}

// NewInputFile creates an InputFile from a generic fs.FS.
// path must be a forward-slash separated path relative to the root of fsys.
func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	return InputFile{
		Path:    path,
		AbsPath: path, // For virtual FS, AbsPath is usually just the path
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Open: func() (io.ReadCloser, error) {
			return fsys.Open(path)
		},
	}
}

// NewBufferInputFile creates an InputFile from an in-memory byte slice.
func NewBufferInputFile(name string, data []byte) InputFile {
	return InputFile{
		Path:    name,
		AbsPath: name,
		Size:    int64(len(data)),
		ModTime: time.Now(),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}
