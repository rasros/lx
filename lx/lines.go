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

	if head <= 0 && tail <= 0 {
		return data, total
	}

	if head >= total || tail >= total || (head > 0 && tail > 0 && head+tail >= total) {
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

// splitLines splits data into logical lines, trimming the last empty chunk
// when the file ends with a newline.
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

// sliceLines returns data restricted by head/tail settings.
func sliceLines(data []byte, head, tail int) []byte {
	view, _ := prepareView(data, head, tail)
	return view
}

// addLineNumbers prefixes each logical line with "N: ".
// Ellipsis lines ("... (N rows skipped)") are left unnumbered.
func addLineNumbers(data []byte, totalRows, head, tail int) []byte {
	if len(data) == 0 {
		return data
	}

	lines := splitLines(data)
	if len(lines) == 0 {
		return data
	}

	numberLines := func(lines [][]byte, startLine int) []byte {
		buf := make([]byte, 0, len(data)+len(lines)*8)
		lineNum := startLine
		for _, ln := range lines {
			num := strconv.Itoa(lineNum)
			buf = append(buf, []byte(num)...)
			buf = append(buf, ':', ' ')
			buf = append(buf, ln...)
			lineNum++
		}
		return buf
	}

	fullFile := (head <= 0 && tail <= 0) ||
		head >= totalRows ||
		tail >= totalRows ||
		(head > 0 && tail > 0 && head+tail >= totalRows)

	if fullFile {
		return numberLines(lines, 1)
	}

	switch {
	case head > 0 && tail > 0 && head+tail < totalRows:
		buf := make([]byte, 0, len(data)+len(lines)*8)

		if head > len(lines) {
			head = len(lines)
		}
		if head > 0 {
			buf = append(buf, numberLines(lines[:head], 1)...)
		}

		if head < len(lines) {
			buf = append(buf, lines[head]...)
		}

		if head+1 < len(lines) {
			startOrig := totalRows - tail + 1
			bufTail := numberLines(lines[head+1:], startOrig)
			buf = append(buf, bufTail...)
		}

		return buf

	case head > 0 && (tail <= 0):
		return numberLines(lines, 1)

	case tail > 0 && (head <= 0):
		startOrig := totalRows - tail + 1
		return numberLines(lines, startOrig)
	}

	return numberLines(lines, 1)
}
