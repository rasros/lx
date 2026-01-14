package lx

import (
	"bytes"
	"io"
	"os"
	"time"
)

// InputFile represents a file discovered by the Walker or provided via Stdin.
// It abstracts the difference between a real file on disk and virtual content,
// allowing lazy opening of file streams.
type InputFile struct {
	Path      string
	AbsPath   string
	Size      int64
	ModTime   time.Time
	LoadError error // Non-nil if the file could not be found/stat-ed during discovery

	// Open returns a ReadCloser for the file content.
	// The caller is responsible for closing it.
	Open func() (io.ReadCloser, error)
}

// NewOsInputFile creates an InputFile from a real OS path.
func NewOsInputFile(path, absPath string, info os.FileInfo) InputFile {
	return InputFile{
		Path:    path,
		AbsPath: absPath,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Open: func() (io.ReadCloser, error) {
			return os.Open(path)
		},
	}
}

// NewBufferInputFile creates an InputFile from an in-memory byte slice.
// Useful for processing stdin or generated content.
func NewBufferInputFile(name string, data []byte) InputFile {
	return InputFile{
		Path:    name,
		AbsPath: name, // usually just a label like "stdin"
		Size:    int64(len(data)),
		ModTime: time.Now(),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}
