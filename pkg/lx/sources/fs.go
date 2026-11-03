package sources

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"time"
)

type InputFile struct {
	Path string
	// AbsPath is the absolute path on disk, or the URL. It is empty for an
	// archive entry.
	AbsPath string
	// Archive and Entry name an archive entry: the archive's absolute path
	// or URL, and the path the entry is stored at inside it.
	Archive string
	Entry   string
	Size    int64
	ModTime time.Time

	// Open opens the content. A URL gives up with ctx's error once ctx is
	// done.
	Open func(ctx context.Context) (io.ReadCloser, error)

	// mediaType is set by Open from an HTTP Content-Type. It is a pointer so
	// copies of the InputFile see it.
	mediaType *string
}

// Key names the input uniquely within a run, as core.Failure.Path does.
func (f InputFile) Key() string {
	if f.Archive != "" {
		return f.Archive + "/" + f.Entry
	}
	return f.AbsPath
}

type byteReaderReadCloser struct {
	*bytes.Reader
}

func (r byteReaderReadCloser) Close() error              { return nil }
func (r byteReaderReadCloser) ByteReader() *bytes.Reader { return r.Reader }

func newByteReaderReadCloser(data []byte) io.ReadCloser {
	return byteReaderReadCloser{Reader: bytes.NewReader(data)}
}

// irregular are the file types that can block and may never end.
const irregular = fs.ModeNamedPipe | fs.ModeSocket | fs.ModeDevice

// IsIrregular reports whether mode is a FIFO, a socket or a device, which is
// read as a Stream of unknown size.
func IsIrregular(mode fs.FileMode) bool { return mode&irregular != 0 }

// DefaultStreamLimit is how much of a stream is read when nothing sets its
// limit: a device such as /dev/zero never ends. It is decimal, as -m is.
const DefaultStreamLimit = 64_000_000

// NewInputFile returns the file at path in fsys. An irregular file, such as a
// FIFO, a device or a process substitution's /dev/fd entry, has an unknown
// size (-1) and opens as a Stream.
func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	f := InputFile{
		Path:    path,
		AbsPath: path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Open:    func(context.Context) (io.ReadCloser, error) { return fsys.Open(path) },
	}
	if IsIrregular(info.Mode()) {
		f.Size = -1
		f.Open = func(context.Context) (io.ReadCloser, error) {
			sf, err := fsys.Open(path)
			if err != nil {
				return nil, err
			}
			return &Stream{sf}, nil
		}
	}
	return f
}

// Stream is an opened input that has no size, reads only forward and may
// never end, such as a FIFO or a device. It is read with ReadStream.
type Stream struct{ fs.File }

// ReadStream reads r to its end, up to limit bytes, or DefaultStreamLimit when
// limit is not positive, and fails when r goes on past it.
func ReadStream(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = DefaultStreamLimit
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err == nil && int64(len(data)) > limit {
		return nil, fmt.Errorf("stopped reading after %d bytes (-m sets the limit)", limit)
	}
	return data, err
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
		Open: func(context.Context) (io.ReadCloser, error) {
			return newByteReaderReadCloser(data), nil
		},
	}
}
