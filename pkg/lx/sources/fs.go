package sources

import (
	"bytes"
	"io"
	"io/fs"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
)

type InputFile struct {
	Path      string
	AbsPath   string
	Size      int64
	ModTime   time.Time
	LoadError error

	Config core.RunnerConfig

	Open func() (io.ReadCloser, error)
}

func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	return InputFile{
		Path:    path,
		AbsPath: path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Open: func() (io.ReadCloser, error) {
			return fsys.Open(path)
		},
	}
}

func NewInputFileFromPath(fsys fs.FS, path string) (InputFile, error) {
	info, err := fs.Stat(fsys, path)
	if err != nil {
		return InputFile{}, err
	}
	return NewInputFile(fsys, path, info), nil
}

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
