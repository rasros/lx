package lx

import (
	"bytes"
	"fmt"
	"math"
)

// countLines counts the number of newline-terminated rows.
// Files without a trailing newline still count their last row.
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

// findNthNewline finds the index *after* the Nth newline.
// If n is larger than total newlines, returns len(data).
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

// prepareView computes the sliced view of data based on head/tail and returns
// both the view and the total number of logical rows in the original data.
func prepareView(data []byte, head, tail int) ([]byte, int) {
	if len(data) == 0 {
		return data, 0
	}

	total := countLines(data)

	// Fast path: if limits exceed total, return original slice
	if (head <= 0 && tail <= 0) || head >= total || tail >= total || (head > 0 && tail > 0 && head+tail >= total) {
		return data, total
	}

	var buf bytes.Buffer

	// Write Head
	if head > 0 {
		limit := findNthNewline(data, head)
		buf.Write(data[:limit])
	}

	// Write Ellipsis
	if head > 0 && tail > 0 {
		skipped := total - head - tail
		if skipped > 0 {
			fmt.Fprintf(&buf, "... (%d rows skipped)\n", skipped)
		}
	}

	// Write Tail
	if tail > 0 {
		linesToSkip := total - tail
		if linesToSkip > 0 {
			start := findNthNewline(data, linesToSkip)
			buf.Write(data[start:])
		} else {
			buf.Write(data)
		}
	}

	return buf.Bytes(), total
}

func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// addLineNumbers prefixes each logical line with "N: ".
func addLineNumbers(data []byte, totalRows, head, tail int) []byte {
	if len(data) == 0 {
		return data
	}

	lines := splitLines(data)
	if len(lines) == 0 {
		return data
	}

	// Calculate padding width: log10(total) + 1
	width := 1
	if totalRows > 9 {
		width = int(math.Log10(float64(totalRows))) + 1
	}
	fmtStr := fmt.Sprintf("%%%dd: ", width)

	numberLines := func(chunk [][]byte, start int) []byte {
		// Estimate capacity to avoid re-allocations
		estSize := len(data) + len(chunk)*(width+2)
		buf := make([]byte, 0, estSize)

		for i, ln := range chunk {
			prefix := fmt.Sprintf(fmtStr, start+i)
			buf = append(buf, []byte(prefix)...)
			buf = append(buf, ln...)
		}
		return buf
	}

	isPartial := head > 0 && tail > 0 && head+tail < totalRows
	isTailOnly := tail > 0 && head <= 0

	if isPartial {
		// Split numbering: 1..head, then ellipsis (unnumbered), then (total-tail+1)..total
		if head > len(lines) {
			head = len(lines)
		}

		var buf []byte
		if head > 0 {
			buf = append(buf, numberLines(lines[:head], 1)...)
		}
		if head < len(lines) {
			// This represents the ellipsis line added by prepareView
			buf = append(buf, lines[head]...)
		}
		if head+1 < len(lines) {
			start := totalRows - tail + 1
			buf = append(buf, numberLines(lines[head+1:], start)...)
		}
		return buf
	}

	start := 1
	if isTailOnly {
		start = totalRows - tail + 1
	}

	return numberLines(lines, start)
}
