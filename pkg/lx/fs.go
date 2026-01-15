package lx

import (
	"bytes"
	"io"
	"os"
	"time"
)

// InputFile represents a file to be processed.
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
