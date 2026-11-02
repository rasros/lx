package internal

import (
	"bytes"
	"errors"
	"io"
	"slices"
)

// lineChunk is how much a LineSource reads at a time when it only needs part
// of the content.
const lineChunk = 64 * 1024

// ExactCountLimit is the largest content Count will scan in full for an exact
// line count when the lines have not already been read. Beyond it, Count
// extrapolates from the first chunk and reports the result as an estimate.
const ExactCountLimit int64 = 64 << 20

// LineSource reads lines from random-access content of a known size. Lines
// end at '\n' and are returned byte-for-byte, so "\r\n" endings survive and no
// line is ever too long to read. A line is counted the way CountLines does: a
// final line without a trailing newline still counts.
//
// Whatever reads the content in full keeps it, so later calls slice it from
// memory and Count is exact without another pass.
type LineSource struct {
	r     io.ReaderAt
	size  int64
	data  []byte // the whole content, once some read has covered it
	count int    // exact line count, or -1 until known
}

// NewLineSource reads lines from the first size bytes of r. A negative size is
// treated as empty.
func NewLineSource(r io.ReaderAt, size int64) *LineSource {
	return &LineSource{r: r, size: max(size, 0), count: -1}
}

// Size is the number of bytes the source covers. It shrinks if the content
// turned out shorter than promised.
func (s *LineSource) Size() int64 { return s.size }

// All returns the whole content.
func (s *LineSource) All() ([]byte, error) {
	if s.data != nil || s.size == 0 {
		return s.data, nil
	}
	buf := make([]byte, s.size)
	n, err := s.readAt(buf, 0)
	if err != nil {
		return nil, err
	}
	s.keep(buf[:n])
	return s.data, nil
}

// Head returns the first n lines, or all of them when n is negative.
func (s *LineSource) Head(n int) ([]byte, error) {
	if n < 0 {
		return s.All()
	}
	if n == 0 || s.size == 0 {
		return nil, nil
	}
	if s.data != nil {
		return headOf(s.data, n), nil
	}

	var buf []byte
	var off int64
	scanned, found := 0, 0
	for off < s.size {
		want := int(min(int64(lineChunk), s.size-off))
		buf = slices.Grow(buf, want)
		got, err := s.readAt(buf[len(buf):len(buf)+want], off)
		if err != nil {
			return nil, err
		}
		buf = buf[:len(buf)+got]
		off += int64(got)
		for found < n {
			i := bytes.IndexByte(buf[scanned:], '\n')
			if i < 0 {
				scanned = len(buf)
				break
			}
			scanned += i + 1
			found++
		}
		if found == n {
			if off >= s.size {
				s.keep(buf)
			}
			return buf[:scanned], nil
		}
	}
	s.keep(buf)
	return buf, nil
}

// Tail returns the last n lines.
func (s *LineSource) Tail(n int) ([]byte, error) {
	if n <= 0 || s.size == 0 {
		return nil, nil
	}
	if s.data != nil {
		return tailOf(s.data, n), nil
	}

	// The tail starts after the n-th newline from the end, not counting a
	// final newline, which ends the last line rather than starting one.
	var last [1]byte
	if err := s.readFull(last[:], s.size-1); err != nil {
		return nil, err
	}
	need := n
	if last[0] == '\n' {
		need++
	}

	var chunks [][]byte // read back to front
	off := s.size
	start := -1 // where the tail starts within the last chunk read
	for off > 0 && start < 0 {
		want := min(int64(lineChunk), off)
		off -= want
		chunk := make([]byte, want)
		if err := s.readFull(chunk, off); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
		end := len(chunk)
		for need > 0 {
			i := bytes.LastIndexByte(chunk[:end], '\n')
			if i < 0 {
				break
			}
			end = i
			if need--; need == 0 {
				start = i + 1
			}
		}
	}

	var joined []byte
	for i := len(chunks) - 1; i >= 0; i-- {
		joined = append(joined, chunks[i]...)
	}
	if off == 0 {
		s.keep(joined)
	}
	return joined[max(start, 0):], nil
}

// Count returns the number of lines and whether that number is exact. It is
// exact whenever the content has been read in full, or is small enough
// (ExactCountLimit) to scan; otherwise it is extrapolated from the first chunk.
func (s *LineSource) Count() (int, bool, error) {
	if s.count >= 0 {
		return s.count, true, nil
	}
	if s.size > ExactCountLimit {
		return s.estimate()
	}

	buf := make([]byte, min(int64(lineChunk), s.size))
	var off int64
	var lastByte byte
	newlines := 0
	for off < s.size {
		want := min(int64(len(buf)), s.size-off)
		got, err := s.readAt(buf[:want], off)
		if err != nil {
			return 0, false, err
		}
		if got == 0 {
			break
		}
		newlines += bytes.Count(buf[:got], []byte{'\n'})
		lastByte = buf[got-1]
		off += int64(got)
	}
	s.count = newlines
	if off > 0 && lastByte != '\n' {
		s.count++
	}
	return s.count, true, nil
}

// estimate extrapolates the line count of content too large to scan from the
// average line length of its first chunk.
func (s *LineSource) estimate() (int, bool, error) {
	buf := make([]byte, lineChunk)
	got, err := s.readAt(buf, 0)
	if err != nil {
		return 0, false, err
	}
	newlines := bytes.Count(buf[:got], []byte{'\n'})
	if newlines == 0 {
		return 1, false, nil
	}
	return int(s.size * int64(newlines) / int64(got)), false, nil
}

// keep records content that a read covered in full.
func (s *LineSource) keep(data []byte) {
	if data == nil {
		data = []byte{}
	}
	s.data = data
	s.size = int64(len(data))
	s.count = CountLines(data)
}

// readAt fills as much of buf as the content holds from off. Content that
// ends early is not an error: the source shrinks to what was there.
func (s *LineSource) readAt(buf []byte, off int64) (int, error) {
	n, err := s.r.ReadAt(buf, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	if n < len(buf) {
		s.size = off + int64(n)
	}
	return n, nil
}

// readFull reads exactly len(buf) bytes at off. Reading back from the end
// relies on the size being right, so content that ends early is an error.
func (s *LineSource) readFull(buf []byte, off int64) error {
	n, err := s.r.ReadAt(buf, off)
	if n == len(buf) {
		return nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// headOf returns the first n lines of data.
func headOf(data []byte, n int) []byte {
	pos := 0
	for ; n > 0; n-- {
		i := bytes.IndexByte(data[pos:], '\n')
		if i < 0 {
			return data
		}
		pos += i + 1
	}
	return data[:pos]
}

// tailOf returns the last n lines of data.
func tailOf(data []byte, n int) []byte {
	end := len(data)
	if end > 0 && data[end-1] == '\n' {
		end--
	}
	for ; n > 0; n-- {
		i := bytes.LastIndexByte(data[:end], '\n')
		if i < 0 {
			return data
		}
		end = i
	}
	return data[end+1:]
}
