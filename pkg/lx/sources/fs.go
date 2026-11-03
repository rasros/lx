package sources

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"time"
)

type InputFile struct {
	Path    string
	AbsPath string
	Size    int64
	ModTime time.Time

	// Open opens the content. Once ctx is done, an Open still waiting gives
	// up with ctx's error, and the reads of what it opened stop with an
	// error soon after, so a cancelled run never waits on a slow source.
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
// waiting on whatever is at the other end.
const irregular = fs.ModeNamedPipe | fs.ModeSocket | fs.ModeDevice | fs.ModeCharDevice

func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	open := func(context.Context) (io.ReadCloser, error) { return fsys.Open(path) }
	if info.Mode()&irregular != 0 {
		open = func(ctx context.Context) (io.ReadCloser, error) { return openBlocking(ctx, fsys, path) }
	}
	return InputFile{
		Path:    path,
		AbsPath: path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Open:    open,
	}
}

// openBlocking opens a file whose open and reads may block, such as a FIFO
// with no writer, so that ctx interrupts both. An open that ctx abandons
// finishes on its own and closes what it opened. Once ctx is done the file
// is closed under its reader, which for an OS pipe or device ends a read in
// progress.
func openBlocking(ctx context.Context, fsys fs.FS, path string) (io.ReadCloser, error) {
	type opened struct {
		f   fs.File
		err error
	}
	ch := make(chan opened, 1)
	go func() {
		f, err := fsys.Open(path)
		ch <- opened{f, err}
	}()
	select {
	case o := <-ch:
		if o.err != nil {
			return nil, o.err
		}
		return &blockingFile{ctx: ctx, f: o.f, stop: context.AfterFunc(ctx, func() { _ = o.f.Close() })}, nil
	case <-ctx.Done():
		go func() {
			if o := <-ch; o.err == nil {
				_ = o.f.Close()
			}
		}()
		return nil, ctx.Err()
	}
}

// blockingFile is a file that ctx closes when it is done. Its reads then fail
// with ctx's error rather than the closed file's.
type blockingFile struct {
	ctx  context.Context
	f    fs.File
	stop func() bool
}

func (b *blockingFile) Read(p []byte) (int, error) {
	n, err := b.f.Read(p)
	return n, b.cause(err)
}

// ReadAt reads as the file does, which keeps such a file read as it was
// before ctx could close it: it states a size of zero, so nothing is read
// at offsets, and a device that never ends is not read to its end.
func (b *blockingFile) ReadAt(p []byte, off int64) (int, error) {
	ra, ok := b.f.(io.ReaderAt)
	if !ok {
		return 0, errors.ErrUnsupported
	}
	n, err := ra.ReadAt(p, off)
	return n, b.cause(err)
}

func (b *blockingFile) cause(err error) error {
	if err != nil {
		if ctxErr := b.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
	}
	return err
}

func (b *blockingFile) Close() error {
	if !b.stop() {
		// ctx closed it already.
		return nil
	}
	return b.f.Close()
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
