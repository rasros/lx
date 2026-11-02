package internal

import (
	"errors"
	"io"
	"slices"
	"sync"
)

const (
	// lazyChunk is the least a LazyReader asks its stream for at a time, so
	// that many small reads do not each become a read of the stream.
	lazyChunk = 32 << 10
	// lazyPrefixCap is how much of the start a LazyReader keeps while gaps are
	// being skipped. It holds every header a parser or sniffer reads twice.
	lazyPrefixCap = 1 << 20
	// lazyWindowCap bounds the window kept past the prefix while gaps are being
	// skipped; once it is exceeded, all but the last lazyWindowKeep bytes
	// before the current read are dropped.
	lazyWindowCap  = 4 << 20
	lazyWindowKeep = 256 << 10
)

var (
	// ErrLazyDiscarded reports a read of bytes a LazyReader skipped over and so
	// no longer has.
	ErrLazyDiscarded = errors.New("lazy reader: bytes already skipped")
	errLazyClosed    = errors.New("lazy reader: closed")
)

// LazyReader gives random access to a stream that can only be read forward,
// reading it on demand up to the furthest offset asked for. A consumer that
// needs only a header leaves the rest of the stream unread; one that needs
// everything reads it all, and every byte read is kept.
//
// While a header-only consumer has asked to skip gaps (skipGaps), a read that
// lands past what is buffered and past the first lazyPrefixCap bytes discards
// the bytes in between instead of keeping them, and the window kept beyond the
// prefix slides with the reads. Memory then stays bounded however far into the
// stream a parser walks, at the cost of those bytes, which later reads cannot
// have back (ErrLazyDiscarded).
//
// It is safe for concurrent use.
type LazyReader struct {
	mu       sync.Mutex
	src      io.Reader
	closer   io.Closer
	size     int64 // declared or discovered length, -1 until known
	pos      int64 // bytes taken from src
	prefix   []byte
	windowed bool   // whether bytes have been dropped, so prefix no longer reaches pos
	win      []byte // [winOff, pos) once windowed
	winOff   int64
	skipping int
	eof      bool
	err      error // the stream's failure, other than a clean end
	closed   bool
}

// NewLazyReader reads src on demand. A non-negative size is the length the
// stream declares, such as an HTTP Content-Length: Size reports it without
// reading anything, the stream is never read past it, and a stream that ends
// sooner fails with io.ErrUnexpectedEOF. A negative size leaves the length to
// be found by reading to the end.
func NewLazyReader(src io.Reader, size int64) *LazyReader {
	r := &LazyReader{src: src, size: -1}
	if c, ok := src.(io.Closer); ok {
		r.closer = c
	}
	if size >= 0 {
		r.size = size
		r.src = io.LimitReader(src, size)
	}
	return r
}

// Size returns the stream's length, reading the stream to its end when the
// length was not declared.
func (r *LazyReader) Size() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size < 0 {
		if r.windowed {
			r.discard(-1)
		} else {
			r.fillPrefix(-1)
		}
		r.size = r.pos
	}
	return r.size
}

// Close closes the stream without draining it. Bytes already buffered stay
// readable; nothing more is read.
func (r *LazyReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

// Consumed returns how many bytes have been taken from the stream so far.
func (r *LazyReader) Consumed() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pos
}

// skipGaps lets reads discard the stream between what is buffered and where
// they start, for a consumer that reads headers scattered through a stream it
// does not need whole. It returns the function that ends it.
func (r *LazyReader) skipGaps() (done func()) {
	r.mu.Lock()
	r.skipping++
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		r.skipping--
		r.mu.Unlock()
	}
}

func (r *LazyReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("lazy reader: negative offset")
	}
	if len(p) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	end := off + int64(len(p))
	if r.size >= 0 && end > r.size {
		end = r.size
	}
	if off < end {
		if err := r.ensure(off, end); err != nil {
			return 0, err
		}
	}
	n := r.copyOut(p, off)
	if n < len(p) {
		return n, r.shortErr()
	}
	return n, nil
}

// ensure buffers [off, end), or as much of it as the stream holds.
func (r *LazyReader) ensure(off, end int64) error {
	if end <= int64(len(r.prefix)) {
		return nil
	}
	if !r.windowed {
		if r.skipping == 0 || off < int64(len(r.prefix)) || end <= lazyPrefixCap {
			r.fillPrefix(end)
			return nil
		}
		r.windowed = true
		r.winOff = r.pos
		r.win = nil
	}
	if max(off, int64(len(r.prefix))) < r.winOff {
		return ErrLazyDiscarded
	}
	if r.skipping > 0 {
		switch {
		case off > r.pos:
			r.win = r.win[:0]
			r.discard(off - r.pos)
			r.winOff = r.pos
		case len(r.win) > lazyWindowCap:
			cut := max(r.winOff, off-lazyWindowKeep)
			r.win = append(r.win[:0], r.win[cut-r.winOff:]...)
			r.winOff = cut
		}
	}
	r.win = r.fill(r.win, end)
	return nil
}

// copyOut copies what is buffered from off into p.
func (r *LazyReader) copyOut(p []byte, off int64) int {
	n := 0
	if off < int64(len(r.prefix)) {
		n = copy(p, r.prefix[off:])
	}
	if n < len(p) && r.windowed {
		o := off + int64(n)
		if o >= r.winOff && o < r.winOff+int64(len(r.win)) {
			n += copy(p[n:], r.win[o-r.winOff:])
		}
	}
	return n
}

func (r *LazyReader) shortErr() error {
	switch {
	case r.err != nil:
		return r.err
	case r.closed && !r.eof:
		return errLazyClosed
	}
	return io.EOF
}

func (r *LazyReader) fillPrefix(end int64) {
	if end >= 0 && end == r.size {
		r.prefix = slices.Grow(r.prefix, int(end)-len(r.prefix))
	}
	r.prefix = r.fill(r.prefix, end)
}

// fill appends to buf what the stream holds up to end, or to its end when end
// is negative, and returns buf.
func (r *LazyReader) fill(buf []byte, end int64) []byte {
	for (end < 0 || r.pos < end) && r.readable() {
		want := lazyChunk
		if end >= 0 && end-r.pos > int64(want) {
			want = int(end - r.pos)
		}
		buf = slices.Grow(buf, want)
		n, err := r.src.Read(buf[len(buf) : len(buf)+want])
		buf = buf[:len(buf)+n]
		r.pos += int64(n)
		r.noteErr(err)
	}
	return buf
}

// discard reads n bytes from the stream without keeping them, or all that is
// left when n is negative.
func (r *LazyReader) discard(n int64) {
	var scratch [lazyChunk]byte
	for n != 0 && r.readable() {
		want := int64(len(scratch))
		if n > 0 && n < want {
			want = n
		}
		got, err := r.src.Read(scratch[:want])
		r.pos += int64(got)
		if n > 0 {
			n -= int64(got)
		}
		r.noteErr(err)
	}
}

func (r *LazyReader) readable() bool {
	return !r.eof && r.err == nil && !r.closed
}

func (r *LazyReader) noteErr(err error) {
	switch {
	case err == nil:
	case errors.Is(err, io.EOF):
		r.eof = true
		if r.size >= 0 && r.pos < r.size {
			r.err = io.ErrUnexpectedEOF
		}
	default:
		r.err = err
	}
}
