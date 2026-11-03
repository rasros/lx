package sources

import (
	"errors"
	"io"
)

// content is an archive entry's bytes, held in the chunks they were read in
// until the run renders them. Reading them in place, not joined into one
// slice, keeps an entry's peak memory to its size and one chunk.
type content struct {
	chunks [][]byte
	size   int64
}

const (
	// firstChunk is the most readEntry allocates before content arrives.
	firstChunk = 64 << 10
	// maxChunk bounds the chunks content is read in.
	maxChunk = 8 << 20
	// maxSlack is the unused capacity a last chunk may keep; one with more
	// is copied to its size.
	maxSlack = 4 << 10
)

// readEntry reads an entry whose size is sizeHint, or unknown when
// negative, but no more than limit bytes when limit is positive. It returns
// the content, or, for an entry over limit, none and a size of limit+1,
// since reading stopped there.
//
// The hint, which an archive may lie about, buys nothing up front: chunks
// start at firstChunk, or the hint and a byte when smaller, and double up
// to maxChunk as content arrives, the last one sized to what the hint
// leaves so honest content fits exactly. A last chunk content does not
// fill is copied to its size, so what is held tracks what was read. An
// entry that ends before its claimed size, or whose reader fails, fails.
func readEntry(r io.Reader, sizeHint, limit int64) (*content, int64, error) {
	if limit > 0 {
		r = io.LimitReader(r, limit+1)
	}
	c := &content{}
	size := firstChunk
	if sizeHint >= 0 && sizeHint < firstChunk {
		// One byte over the hint finds the end in the same read.
		size = int(sizeHint) + 1
	}
	var err error
	for {
		if left := sizeHint + 1 - c.size; sizeHint >= 0 && left > 0 && left < int64(size) {
			size = int(left)
		}
		chunk := make([]byte, size)
		var n int
		n, err = fill(r, chunk)
		if n > 0 {
			chunk = chunk[:n]
			if cap(chunk)-n > maxSlack {
				chunk = append(make([]byte, 0, n), chunk...)
			}
			c.chunks = append(c.chunks, chunk)
			c.size += int64(n)
		}
		if err != nil {
			break
		}
		size = min(2*size, maxChunk)
	}
	if limit > 0 && c.size > limit {
		return nil, limit + 1, nil
	}
	if err != io.EOF {
		return nil, c.size, err
	}
	return c, c.size, nil
}

// fill reads into buf until it is full or r fails, with io.EOF at the end
// of r's content.
func fill(r io.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := r.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// ReadAt reads the content at off, as io.ReaderAt does.
func (c *content) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	n := 0
	for _, chunk := range c.chunks {
		if off >= int64(len(chunk)) {
			off -= int64(len(chunk))
			continue
		}
		m := copy(p[n:], chunk[off:])
		n += m
		off = 0
		if n == len(p) {
			return n, nil
		}
	}
	return n, io.EOF
}

// open returns a reader of the content, which is also its io.ReaderAt.
func (c *content) open() io.ReadCloser {
	if c == nil {
		c = &content{}
	}
	return &contentReader{SectionReader: io.NewSectionReader(c, 0, c.size)}
}

// contentReader reads a content from its start, and at any offset; Size
// reports its size, which readers of an io.ReaderAt need.
type contentReader struct {
	*io.SectionReader
}

func (contentReader) Close() error { return nil }
