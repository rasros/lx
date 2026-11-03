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
	Path    string
	AbsPath string
	Size    int64
	ModTime time.Time

	// Open opens the content. A source that can block, such as a URL, a FIFO
	// or a device, gives up with ctx's error once ctx is done, in its open and
	// in the reads of what it opened. A regular file or an input already in
	// memory need not watch ctx: it cannot keep a cancelled run waiting.
	Open func(ctx context.Context) (io.ReadCloser, error)

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

// irregular are the file types whose open and reads can block indefinitely,
// waiting on whatever is at the other end, and that may never end. Their
// stated size, zero, says nothing of their content.
const irregular = fs.ModeNamedPipe | fs.ModeSocket | fs.ModeDevice

// DefaultStreamLimit is how much of a Stream is read when nothing sets its
// limit: a device such as /dev/zero never ends.
const DefaultStreamLimit = 64 << 20

// NewInputFile returns the file at path in fsys. An irregular file, such as a
// FIFO, a device or a process substitution's /dev/fd entry, opens as a Stream,
// and both its open and its reads stop when ctx is done. An open of a FIFO that
// no writer ever opens cannot itself be stopped: its goroutine and OS thread
// stay blocked for the life of the process.
func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	open := func(context.Context) (io.ReadCloser, error) { return fsys.Open(path) }
	if info.Mode()&irregular != 0 {
		open = func(ctx context.Context) (io.ReadCloser, error) { return openStream(ctx, fsys, path) }
	}
	return InputFile{
		Path:    path,
		AbsPath: path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Open:    open,
	}
}

func openStream(ctx context.Context, fsys fs.FS, path string) (*Stream, error) {
	f, err := openUntilDone(ctx, func() (fs.File, error) { return fsys.Open(path) })
	if err != nil {
		return nil, err
	}
	return &Stream{
		ctx:   ctx,
		f:     f,
		stop:  context.AfterFunc(ctx, func() { _ = f.Close() }),
		limit: DefaultStreamLimit,
	}, nil
}

// Stream is an opened input that has no size, reads only forward and may
// never end, such as a FIFO or a device. It reads up to a limit, and fails
// with a *StreamLimitError past it. Once ctx is done the file is closed under
// its read, which for an OS pipe or device ends a read in progress, and the
// read fails with ctx's error.
type Stream struct {
	ctx      context.Context
	f        fs.File
	stop     func() bool
	limit    int64
	limitSet bool
	n        int64
	err      error
}

// SetLimit sets how many bytes the stream reads, in place of
// DefaultStreamLimit, before any is read.
func (s *Stream) SetLimit(n int64) {
	s.limit, s.limitSet = n, true
}

// Err returns the *StreamLimitError of a stream that went past its limit.
func (s *Stream) Err() error { return s.err }

func (s *Stream) Read(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if s.n >= s.limit {
		// One byte more tells a stream that ended at the limit from one
		// that goes on past it.
		var one [1]byte
		n, err := s.f.Read(one[:])
		if n > 0 {
			s.err = &StreamLimitError{Limit: s.limit, Default: !s.limitSet}
			return 0, s.err
		}
		return 0, s.cause(err)
	}
	if rest := s.limit - s.n; int64(len(p)) > rest {
		p = p[:rest]
	}
	n, err := s.f.Read(p)
	s.n += int64(n)
	return n, s.cause(err)
}

func (s *Stream) cause(err error) error {
	if err != nil {
		if ctxErr := s.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
	}
	return err
}

func (s *Stream) Close() error {
	if !s.stop() {
		// ctx closed it already.
		return nil
	}
	return s.f.Close()
}

// StreamLimitError is a Stream that went on past its limit.
type StreamLimitError struct {
	Limit int64
	// Default is whether the limit is DefaultStreamLimit, which nothing set.
	Default bool
}

func (e *StreamLimitError) Error() string {
	if e.Default {
		return fmt.Sprintf("stopped reading after %d bytes, the default limit for a stream (-m sets another)", e.Limit)
	}
	return fmt.Sprintf("stopped reading after %d bytes, the size limit (-m)", e.Limit)
}

// openUntilDone returns what open returns, or ctx's error once ctx is done.
// An open that ctx abandons keeps running in its goroutine, which closes what
// it opens should it ever return. One that never returns, such as a FIFO's
// open waiting on a writer that never comes, cannot be reclaimed.
func openUntilDone[T io.Closer](ctx context.Context, open func() (T, error)) (T, error) {
	type opened struct {
		v   T
		err error
	}
	// Unbuffered: a send succeeds only while the caller still waits, so
	// exactly one side ends up owning what was opened.
	ch := make(chan opened)
	go func() {
		v, err := open()
		select {
		case ch <- opened{v, err}:
		case <-ctx.Done():
			if err == nil {
				_ = v.Close()
			}
		}
	}()
	select {
	case o := <-ch:
		return o.v, o.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
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
		Open: func(context.Context) (io.ReadCloser, error) {
			return newByteReaderReadCloser(data), nil
		},
	}
}
