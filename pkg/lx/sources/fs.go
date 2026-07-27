package sources

import (
	"bytes"
	"io"
	"io/fs"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
)

type InputFile struct {
	Path    string
	AbsPath string
	Size    int64
	ModTime time.Time

	Config core.RunnerConfig

	Open func() (io.ReadCloser, error)

	// mediaType is filled in by Open for sources that learn it while fetching,
	// such as an HTTP response's Content-Type. It is shared by pointer so the
	// value copies the pipeline makes all observe what Open recorded.
	mediaType *string
}

type byteReaderReadCloser struct {
	*bytes.Reader
}

func (r byteReaderReadCloser) Close() error              { return nil }
func (r byteReaderReadCloser) ByteReader() *bytes.Reader { return r.Reader }

func newByteReaderReadCloser(data []byte) io.ReadCloser {
	return byteReaderReadCloser{Reader: bytes.NewReader(data)}
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
			return newByteReaderReadCloser(data), nil
		},
	}
}
