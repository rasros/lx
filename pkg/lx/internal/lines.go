package internal

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
)

type LineNumberFormatter struct {
	Head      []byte
	Gap       []byte
	Tail      []byte
	TotalRows int
}

func (lnf LineNumberFormatter) Format(f fmt.State, c rune) {
	lnf.writeTo(f)
}

// String lets consumers that only understand text — template helpers, the token
// counter — see the numbered content instead of falling back to %v.
func (lnf LineNumberFormatter) String() string {
	var buf bytes.Buffer
	buf.Grow(len(lnf.Head) + len(lnf.Gap) + len(lnf.Tail))
	lnf.writeTo(&buf)
	return buf.String()
}

func (lnf LineNumberFormatter) writeTo(out io.Writer) {
	width := 1
	if lnf.TotalRows > 9 {
		width = int(math.Log10(float64(lnf.TotalRows))) + 1
	}

	numBuf := make([]byte, 0, 20)
	spaces := []byte("          ")
	colon := []byte(": ")

	printChunk := func(data []byte, startRow int) {
		currentRow := startRow
		offset := 0
		write := func(b []byte) { out.Write(b) }

		for offset < len(data) {
			idx := bytes.IndexByte(data[offset:], '\n')
			end := offset + idx
			if idx == -1 {
				end = len(data)
			} else {
				end++
			}

			numBuf = strconv.AppendInt(numBuf[:0], int64(currentRow), 10)
			padLen := width - len(numBuf)
			if padLen > 0 {
				for len(spaces) < padLen {
					spaces = append(spaces, ' ')
				}
				write(spaces[:padLen])
			}

			write(numBuf)
			write(colon)
			write(data[offset:end])

			currentRow++
			offset = end
		}
	}

	if len(lnf.Head) > 0 {
		printChunk(lnf.Head, 1)
	}

	if len(lnf.Gap) > 0 {
		out.Write(lnf.Gap)
	}

	if len(lnf.Tail) > 0 {
		tailCount := CountLines(lnf.Tail)
		startRow := lnf.TotalRows - tailCount + 1
		if startRow < 1 {
			startRow = 1
		}
		printChunk(lnf.Tail, startRow)
	}
}

func EstimateLineCount(r io.ReaderAt, fileSize int64, buf []byte) (int, bool, error) {
	if fileSize == 0 {
		return 0, true, nil
	}

	n, err := r.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, false, err
	}

	if int64(n) >= fileSize {
		return CountLines(buf[:n]), true, nil
	}

	newlineCount := bytes.Count(buf[:n], []byte("\n"))
	if newlineCount == 0 {
		return 1, false, nil
	}

	avgLineLen := float64(n) / float64(newlineCount)
	if avgLineLen == 0 {
		return 0, false, nil
	}

	estimated := int(float64(fileSize) / avgLineLen)
	return estimated, false, nil
}

func ReadHead(r io.Reader, n int) ([]byte, int, error) {
	if n == 0 {
		return nil, 0, nil
	}

	var buf bytes.Buffer
	sc := bufio.NewScanner(r)

	scanBuf := make([]byte, 0, 64*1024)
	sc.Buffer(scanBuf, 10*1024*1024)

	linesRead := 0
	readAll := n < 0

	for sc.Scan() {
		buf.Write(sc.Bytes())
		buf.WriteByte('\n')
		linesRead++
		if !readAll && linesRead >= n {
			break
		}
	}
	return buf.Bytes(), linesRead, sc.Err()
}

func ReadTailSeek(f *os.File, linesWanted int) ([]byte, error) {
	if linesWanted <= 0 {
		return nil, nil
	}

	const chunkSize = 4096
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	if fileSize == 0 {
		return nil, nil
	}

	var chunks [][]byte
	linesFound := 0
	offset := fileSize

	for offset > 0 && linesFound < linesWanted {
		readSize := int64(chunkSize)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, offset); err != nil {
			return nil, err
		}

		count := bytes.Count(buf, []byte("\n"))
		linesFound += count

		chunks = append(chunks, buf)
	}

	totalLen := 0
	for _, chunk := range chunks {
		totalLen += len(chunk)
	}

	result := make([]byte, totalLen)
	currentPos := 0

	for i := len(chunks) - 1; i >= 0; i-- {
		copy(result[currentPos:], chunks[i])
		currentPos += len(chunks[i])
	}

	extraLines := linesFound - linesWanted
	if extraLines > 0 {
		idx := findNthNewline(result, extraLines)
		result = result[idx:]
	}

	return result, nil
}

// SplitLines splits src into lines on \n boundaries.
func SplitLines(src []byte) [][]byte {
	return bytes.Split(src, []byte("\n"))
}

// AppendLines appends lines[from..to] (inclusive) to out, each followed by \n.
func AppendLines(out []byte, lines [][]byte, from, to int) []byte {
	for i := from; i <= to && i < len(lines); i++ {
		out = append(out, lines[i]...)
		out = append(out, '\n')
	}
	return out
}

// AppendLine appends lines[row] to out followed by \n.
func AppendLine(out []byte, lines [][]byte, row int) []byte {
	if row >= 0 && row < len(lines) {
		out = append(out, lines[row]...)
		out = append(out, '\n')
	}
	return out
}

func CountLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func findNthNewline(data []byte, n int) int {
	count := 0
	for i, b := range data {
		if b == '\n' {
			count++
			if count == n {
				return i + 1
			}
		}
	}
	return len(data)
}
