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

// maxStreamSize bounds how much of an irregular file is read: a device such
// as /dev/zero never ends.
const maxStreamSize = 64 << 20

// NewInputFile returns the file at path in fsys. An irregular file, such as a
// FIFO, a device or a process substitution's /dev/fd entry, is read as a
// stream when it is opened, up to maxStreamSize, and both the open and the
// read stop when ctx is done. An open of a FIFO that no writer ever opens
// cannot itself be stopped: its goroutine and OS thread stay blocked for the
// life of the process.
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

// openStream reads an irregular file whole, through Read, since it cannot be
// read at offsets. Once ctx is done the file is closed under the read, which
// for an OS pipe or device ends a read in progress.
func openStream(ctx context.Context, fsys fs.FS, path string) (io.ReadCloser, error) {
	f, err := openUntilDone(ctx, func() (fs.File, error) { return fsys.Open(path) })
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = f.Close() })
	data, err := io.ReadAll(io.LimitReader(f, maxStreamSize+1))
	if stop() {
		_ = f.Close()
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	if len(data) > maxStreamSize {
		return nil, fmt.Errorf("stopped reading after %d MiB: the stream may not end", maxStreamSize>>20)
	}
	return newByteReaderReadCloser(data), nil
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
