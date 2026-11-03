package internal

import (
	"bytes"
	"errors"
	"io"
)

// lineChunk is how much a LineSource reads at a time when it only needs part
// of the content.
const lineChunk = 64 * 1024

// ExactCountLimit is the largest content Count scans in full when the lines
// have not been read. Beyond it, Count extrapolates from the first chunk.
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
	data, err := io.ReadAll(io.NewSectionReader(s.r, 0, s.size))
	if err != nil {
		return nil, err
	}
	s.keep(data)
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
	found := 0
	for off := int64(0); off < s.size; {
		chunk := make([]byte, min(int64(lineChunk), s.size-off))
		got, err := s.readAt(chunk, off)
		if err != nil {
			return nil, err
		}
		buf = append(buf, chunk[:got]...)
		off += int64(got)
		if got < len(chunk) {
			break // the content ended early
		}
		found += bytes.Count(chunk, []byte{'\n'})
		if found >= n {
			return headOf(buf, n), nil
		}
	}
	s.keep(buf)
	return headOf(buf, n), nil
}

// Tail returns the last n lines. Content that turns out shorter than the size
// promised is read in full instead.
func (s *LineSource) Tail(n int) ([]byte, error) {
	if n <= 0 || s.size == 0 {
		return nil, nil
	}
	if s.data != nil {
		return tailOf(s.data, n), nil
	}
	// The tail starts after the n-th newline from the end, not counting a
	// final newline, which ends the last line rather than starting one.
	var buf []byte
	for off := s.size; off > 0; {
		want := min(int64(lineChunk), off)
		off -= want
		chunk := make([]byte, want)
		if got, err := s.r.ReadAt(chunk, off); got < len(chunk) {
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}
			all, err := s.All()
			return tailOf(all, n), err
		}
		buf = append(chunk, buf...)
		if bytes.Count(bytes.TrimSuffix(buf, []byte{'\n'}), []byte{'\n'}) >= n {
			return tailOf(buf, n), nil
		}
	}
	s.keep(buf)
	return tailOf(buf, n), nil
}

// Count returns the number of lines and whether that number is exact: it is
// unless the content is over ExactCountLimit and has not been read in full.
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
		got, err := s.readAt(buf[:min(int64(len(buf)), s.size-off)], off)
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
	if got < len(buf) {
		// The content ended within the chunk, and has been read whole.
		s.keep(buf[:got])
		return s.count, true, nil
	}
	newlines := bytes.Count(buf, []byte{'\n'})
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
