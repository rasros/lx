package lx

import (
	"bytes"
	"strconv"
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

// prepareView computes the sliced view of data based on head/tail and returns
// both the view and the total number of logical rows in the original data.
func prepareView(data []byte, head, tail int) ([]byte, int) {
	if len(data) == 0 {
		return data, 0
	}

	lines := splitLines(data)
	total := len(lines)

	if (head <= 0 && tail <= 0) || head >= total || tail >= total || (head > 0 && tail > 0 && head+tail >= total) {
		return data, total
	}

	var out [][]byte

	switch {
	case head > 0 && tail > 0:
		skipped := total - head - tail
		out = append(out, lines[:head]...)
		out = append(out, []byte("... ("+strconv.Itoa(skipped)+" rows skipped)\n"))
		out = append(out, lines[total-tail:]...)

	case head > 0:
		out = lines[:head]

	case tail > 0:
		out = lines[total-tail:]
	}

	return bytes.Join(out, nil), total
}

func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func sliceLines(data []byte, head, tail int) []byte {
	view, _ := prepareView(data, head, tail)
	return view
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

	numberLines := func(chunk [][]byte, start int) []byte {
		buf := make([]byte, 0, len(data)+len(chunk)*8)
		for i, ln := range chunk {
			prefix := strconv.Itoa(start+i) + ": "
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
