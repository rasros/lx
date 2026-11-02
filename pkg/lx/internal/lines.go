package internal

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
)

// LineNumberFormatter prefixes each line of a head, gap and tail with its row
// number. Tail rows are counted back from TotalRows; when that count is an
// estimate, Approximate marks each tail number with "~" so it is not mistaken
// for an exact position.
type LineNumberFormatter struct {
	Head        []byte
	Gap         []byte
	Tail        []byte
	TotalRows   int
	Approximate bool
}

func (lnf LineNumberFormatter) Format(f fmt.State, c rune) {
	lnf.writeTo(f)
}

// String returns the numbered content as text.
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
	if lnf.Approximate && len(lnf.Tail) > 0 {
		width++ // room for the "~" on tail numbers, so the columns line up
	}

	numBuf := make([]byte, 0, 20)
	spaces := []byte("          ")
	colon := []byte(": ")

	printChunk := func(data []byte, startRow int, mark []byte) {
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
			padLen := width - len(mark) - len(numBuf)
			if padLen > 0 {
				for len(spaces) < padLen {
					spaces = append(spaces, ' ')
				}
				write(spaces[:padLen])
			}

			write(mark)
			write(numBuf)
			write(colon)
			write(data[offset:end])

			currentRow++
			offset = end
		}
	}

	if len(lnf.Head) > 0 {
		printChunk(lnf.Head, 1, nil)
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
		var mark []byte
		if lnf.Approximate {
			mark = []byte("~")
		}
		printChunk(lnf.Tail, startRow, mark)
	}
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
