package internal

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
)

// LineNumberFormatter prefixes each line of a head, gap and tail with its row
// number. Tail rows count back from TotalRows, marked "~" when Approximate.
// HeadRows and TailRows, when set, give explicit numbers, as for a skeleton;
// a 0 leaves a line unnumbered.
type LineNumberFormatter struct {
	Head        []byte
	Gap         []byte
	Tail        []byte
	HeadRows    []int
	TailRows    []int
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
	maxRow := lnf.TotalRows
	for _, r := range lnf.HeadRows {
		maxRow = max(maxRow, r)
	}
	for _, r := range lnf.TailRows {
		maxRow = max(maxRow, r)
	}
	width := 1
	if maxRow > 9 {
		width = int(math.Log10(float64(maxRow))) + 1
	}
	approximate := lnf.Approximate && lnf.TailRows == nil
	if approximate && len(lnf.Tail) > 0 {
		width++ // room for the "~" on tail numbers, so the columns line up
	}

	numBuf := make([]byte, 0, 20)
	spaces := []byte("          ")
	colon := []byte(": ")

	// printChunk numbers the lines of data from startRow up, or by rows when
	// that is set.
	printChunk := func(data []byte, startRow int, rows []int, mark []byte) {
		currentRow := startRow
		offset := 0
		write := func(b []byte) { out.Write(b) }

		for line := 0; offset < len(data); line++ {
			idx := bytes.IndexByte(data[offset:], '\n')
			end := offset + idx
			if idx == -1 {
				end = len(data)
			} else {
				end++
			}

			if rows != nil {
				if line < len(rows) {
					currentRow = rows[line]
				}
				if currentRow <= 0 {
					write(data[offset:end])
					offset = end
					continue
				}
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
		printChunk(lnf.Head, 1, lnf.HeadRows, nil)
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
		if approximate {
			mark = []byte("~")
		}
		printChunk(lnf.Tail, startRow, lnf.TailRows, mark)
	}
}

// SplitLines splits src into lines on \n boundaries.
func SplitLines(src []byte) [][]byte {
	return bytes.Split(src, []byte("\n"))
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
