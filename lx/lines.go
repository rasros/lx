package lx

import (
	"bufio"
	"bytes"
	"io"
	"math"
	"os"
	"strconv"
)

// EstimateLineCount reads the first 4KB to calculate an average line length.
func EstimateLineCount(r io.ReaderAt, fileSize int64) (int, error) {
	if fileSize == 0 {
		return 0, nil
	}

	const sampleSize = 4096
	buf := make([]byte, sampleSize)

	n, err := r.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, err
	}

	if int64(n) >= fileSize {
		return countLines(buf[:n]), nil
	}

	newlineCount := bytes.Count(buf[:n], []byte("\n"))
	if newlineCount == 0 {
		return 1, nil
	}

	avgLineLen := float64(n) / float64(newlineCount)
	if avgLineLen == 0 {
		return 0, nil
	}

	estimated := int(float64(fileSize) / avgLineLen)
	return estimated, nil
}

// ReadHead reads the first N lines from a reader.
func ReadHead(r io.Reader, n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	sc := bufio.NewScanner(r)
	linesRead := 0

	for sc.Scan() {
		buf.Write(sc.Bytes())
		buf.WriteByte('\n')
		linesRead++
		if linesRead >= n {
			break
		}
	}
	return buf.Bytes(), sc.Err()
}

// ReadTailSeek reads the last N lines by seeking backwards.
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

	var result []byte
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

		result = append(buf, result...)
	}

	extraLines := linesFound - linesWanted
	if extraLines > 0 {
		idx := findNthNewline(result, extraLines)
		result = result[idx:]
	}

	return result, nil
}

type StreamResult struct {
	HeadBytes []byte
	TailBytes []byte
	TotalRows int
}

func ReadStream(r io.Reader, headLimit, tailLimit int) (StreamResult, error) {
	res := StreamResult{}

	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 10*1024*1024)

	var tailRing []string
	if tailLimit > 0 {
		tailRing = make([]string, 0, tailLimit)
	}

	tailRingIdx := 0
	tailFull := false
	captureAll := headLimit < 0

	for sc.Scan() {
		line := sc.Text()
		res.TotalRows++

		if captureAll || (headLimit > 0 && res.TotalRows <= headLimit) {
			res.HeadBytes = append(res.HeadBytes, []byte(line)...)
			res.HeadBytes = append(res.HeadBytes, '\n')
		}

		if !captureAll && tailLimit > 0 {
			if len(tailRing) < tailLimit {
				tailRing = append(tailRing, line)
			} else {
				tailRing[tailRingIdx] = line
				tailRingIdx = (tailRingIdx + 1) % tailLimit
				tailFull = true
			}
		}
	}

	if err := sc.Err(); err != nil {
		return res, err
	}

	if !captureAll && tailLimit > 0 && len(tailRing) > 0 {
		var tailBuf bytes.Buffer
		count := len(tailRing)
		start := 0
		if tailFull {
			start = tailRingIdx
		}

		for i := 0; i < count; i++ {
			idx := (start + i) % count
			lineNum := res.TotalRows - count + 1 + i

			if lineNum > headLimit {
				tailBuf.WriteString(tailRing[idx])
				tailBuf.WriteByte('\n')
			}
		}
		res.TailBytes = tailBuf.Bytes()
	}

	return res, nil
}

func countLines(data []byte) int {
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

// addLineNumbers optimized for speed on large files.
// It avoids split overhead and fmt.Sprintf.
func addLineNumbers(data []byte, totalRows, head, tail int) []byte {
	if len(data) == 0 {
		return data
	}

	// Calculate padding width
	width := 1
	if totalRows > 9 {
		width = int(math.Log10(float64(totalRows))) + 1
	}

	// Pre-allocate buffer to avoid resizing
	// Estimate: Original Data + (Rows * (Width + 2 chars for ": "))
	lineCount := countLines(data)
	estSize := len(data) + lineCount*(width+2)
	buf := make([]byte, 0, estSize)

	// Determine start row.
	// This simplified logic matches the usage in runner.go for standard reads.
	startRow := 1
	if tail > 0 && head <= 0 {
		startRow = totalRows - tail + 1
	}

	// Reusable buffer for integer conversion
	numBuf := make([]byte, 0, 20)
	spaces := []byte("          ") // pre-allocated spaces for padding
	colon := []byte(": ")

	currentRow := startRow
	offset := 0

	for offset < len(data) {
		// Find next newline
		idx := bytes.IndexByte(data[offset:], '\n')
		end := offset + idx
		if idx == -1 {
			end = len(data)
		} else {
			end++ // Include the newline
		}

		// 1. Format Number
		numBuf = strconv.AppendInt(numBuf[:0], int64(currentRow), 10)

		// 2. Add Padding
		padLen := width - len(numBuf)
		if padLen > 0 {
			// Grow spaces if needed (rare)
			for len(spaces) < padLen {
				spaces = append(spaces, ' ')
			}
			buf = append(buf, spaces[:padLen]...)
		}

		// 3. Write "N: "
		buf = append(buf, numBuf...)
		buf = append(buf, colon...)

		// 4. Write Line Content
		buf = append(buf, data[offset:end]...)

		currentRow++
		offset = end
	}

	return buf
}
