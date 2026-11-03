package sources

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"time"
)

type InputFile struct {
	Path string
	// AbsPath is the absolute path of the file on disk, or the URL of one
	// fetched from a URL. It is empty for an archive entry, which is no file
	// on disk.
	AbsPath string
	// Archive and Entry name an archive entry: the archive's absolute path,
	// or its URL when it was downloaded, and the entry's path inside it.
	// That is the path an entry is stored at, with the directory a
	// repository host wraps a repository's files in, which its Path leaves
	// out; the content of a single compressed file, which has no stored
	// path, has the name its Path gives it, notes.txt for notes.txt.gz.
	Archive string
	Entry   string
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

// Key names the input uniquely among those of one run, as core.Failure.Path
// names one that could not be read: its AbsPath, or its archive's and the
// entry's path in it.
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

// irregular are the file types whose open and reads can block indefinitely,
// waiting on whatever is at the other end, and that may never end. Their
// stated size, zero, says nothing of their content.
const irregular = fs.ModeNamedPipe | fs.ModeSocket | fs.ModeDevice

// IsIrregular reports whether mode is a FIFO, a socket or a device: a file
// whose content is a stream, read as a Stream, with no size known ahead.
func IsIrregular(mode fs.FileMode) bool { return mode&irregular != 0 }

// DefaultStreamLimit is how much of a stream is read when nothing sets its
// limit: a device such as /dev/zero never ends. It is decimal, as -m is.
const DefaultStreamLimit = 64_000_000

// NewInputFile returns the file at path in fsys. An irregular file, such as a
// FIFO, a device or a process substitution's /dev/fd entry, has an unknown
// size (-1) and opens as a Stream, and both its open and its reads stop when
// ctx is done. An open of a FIFO that no writer ever opens cannot itself be
// stopped: its goroutine and OS thread stay blocked for the life of the
// process.
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
		f.Open = func(ctx context.Context) (io.ReadCloser, error) { return openStream(ctx, fsys, path) }
	}
	return f
}

func openStream(ctx context.Context, fsys fs.FS, path string) (*Stream, error) {
	f, err := openUntilDone(ctx, nil, func() (fs.File, error) { return fsys.Open(path) })
	if err != nil {
		return nil, err
	}
	return &Stream{
		ctx:  ctx,
		f:    f,
		stop: context.AfterFunc(ctx, func() { _ = f.Close() }),
		r:    newLimited(f, 0),
	}, nil
}

// Stream is an opened input that has no size, reads only forward and may
// never end, such as a FIFO or a device. It reads up to a limit, and fails
// with a *StreamLimitError past it. Once ctx is done the file is closed under
// its read, which for an OS pipe or device ends a read in progress, and the
// read fails with ctx's error.
type Stream struct {
	ctx  context.Context
	f    fs.File
	stop func() bool
	r    *limited
}

// SetLimit sets how many bytes the stream reads, in place of
// DefaultStreamLimit, before any is read. A limit that is not positive keeps
// the default.
func (s *Stream) SetLimit(n int64) { s.r = newLimited(s.f, n) }

// Err returns the *StreamLimitError of a stream that went past its limit.
func (s *Stream) Err() error { return s.r.err }

func (s *Stream) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if err != nil {
		if ctxErr := s.ctx.Err(); ctxErr != nil {
			return n, ctxErr
		}
	}
	return n, err
}

func (s *Stream) Close() error {
	if !s.stop() {
		// ctx closed it already.
		return nil
	}
	return s.f.Close()
}

// ReadStream reads r to its end as a Stream is read: up to limit, or
// DefaultStreamLimit when limit is not positive, failing with a
// *StreamLimitError past it.
func ReadStream(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(newLimited(r, limit))
}

// limited reads r up to a limit, and fails with a *StreamLimitError once r
// goes on past it.
type limited struct {
	r     io.Reader
	limit int64
	set   bool
	n     int64
	err   error
}

func newLimited(r io.Reader, limit int64) *limited {
	if limit <= 0 {
		return &limited{r: r, limit: DefaultStreamLimit}
	}
	return &limited{r: r, limit: limit, set: true}
}

func (l *limited) Read(p []byte) (int, error) {
	if l.err != nil {
		return 0, l.err
	}
	if l.n >= l.limit {
		// One byte more tells a stream that ended at the limit from one
		// that goes on past it.
		var one [1]byte
		n, err := l.r.Read(one[:])
		if n > 0 {
			l.err = &StreamLimitError{Limit: l.limit, Default: !l.set}
			return 0, l.err
		}
		return 0, err
	}
	if rest := l.limit - l.n; int64(len(p)) > rest {
		p = p[:rest]
	}
	n, err := l.r.Read(p)
	l.n += int64(n)
	return n, err
}

// StreamLimitError is a stream that went on past its limit.
type StreamLimitError struct {
	Limit int64
	// Default is whether the limit is DefaultStreamLimit, which nothing set.
	Default bool
}

func (e *StreamLimitError) Error() string {
	if e.Default {
		return fmt.Sprintf("stopped reading after %s, the default limit for a stream (-m sets another)", formatSize(e.Limit))
	}
	return fmt.Sprintf("stopped reading after %s, the size limit (-m)", formatSize(e.Limit))
}

// formatSize writes n bytes with a decimal unit, as -m reads them and as lx
// reports sizes elsewhere.
func formatSize(n int64) string {
	units := []string{"B", "kB", "MB", "GB", "TB"}
	v, i := float64(n), 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	return strings.TrimSuffix(strconv.FormatFloat(v, 'f', 1, 64), ".0") + " " + units[i]
}

// cleanups counts the abandoned opens that clean up after themselves once
// they finish, which WaitCleanups waits for.
var cleanups sync.WaitGroup

// WaitCleanups waits up to d for work that a cancel abandoned and that
// removes temporary files once it finishes, such as the open of a large
// workbook, and reports whether all of it finished. A process about to exit
// after a cancel calls it so as not to leave those files behind.
func WaitCleanups(d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		cleanups.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// openUntilDone returns what open returns, or ctx's error once ctx is done.
// An open that ctx abandons keeps running in its goroutine, which closes what
// it opens should it ever return. One that never returns, such as a FIFO's
// open waiting on a writer that never comes, cannot be reclaimed. An open
// counted in wg, when it is not nil, is done once its goroutine is.
func openUntilDone[T io.Closer](ctx context.Context, wg *sync.WaitGroup, open func() (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	type opened struct {
		v   T
		err error
	}
	// Unbuffered: a send succeeds only while the caller still waits, so
	// exactly one side ends up owning what was opened.
	ch := make(chan opened)
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
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
