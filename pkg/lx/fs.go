package lx

import (
	"bytes"
	"io"
	"os"
	"time"
)

// InputFile represents a file to be processed.
// It abstracts the difference between a real file on disk and a virtual file.
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

// StdinInputFile represents content from Stdin.
type StdinInputFile struct {
	Content []byte
}

func (s StdinInputFile) ToInputFile() InputFile {
	return InputFile{
		Path:    "stdin",
		AbsPath: "stdin",
		Size:    int64(len(s.Content)),
		ModTime: time.Now(),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(s.Content)), nil
		},
	}
}
