package internal

import "slices"

// ElisionMarker is the line that stands in for source rows left out between
// two kept lines. It echoes the "... (N rows skipped)" gap of a head and tail
// excerpt, without the count: the cut rows are mostly function bodies, a
// count on every one of them would cost more than it tells, and line numbers
// (-l) give the exact positions.
const ElisionMarker = "...\n"

// LineRange keeps the source rows Start to End-1: 0-based indexes into
// SplitLines, half-open. A positive Cut truncates the last row to its first Cut
// bytes, as when a signature shares its line with the body it is cut from.
type LineRange struct {
	Start, End int
	Cut        int
}

// KeepLines appends the rows from..to (inclusive) to out, clipped to lines.
// An empty span appends nothing.
func KeepLines(out []LineRange, lines [][]byte, from, to int) []LineRange {
	from = max(from, 0)
	to = min(to, len(lines)-1)
	if from > to {
		return out
	}
	return append(out, LineRange{Start: from, End: to + 1})
}

// KeepLine appends a single row to out, if lines has it.
func KeepLine(out []LineRange, lines [][]byte, row int) []LineRange {
	return KeepLines(out, lines, row, row)
}

// KeepPrefix appends the first cut bytes of row to out. A cut that is not
// positive or reaches the end of the line keeps the whole line.
func KeepPrefix(out []LineRange, lines [][]byte, row, cut int) []LineRange {
	if row < 0 || row >= len(lines) {
		return out
	}
	if cut >= len(lines[row]) {
		cut = 0
	}
	return append(out, LineRange{Start: row, End: row + 1, Cut: cut})
}

// KeptLines is the text of the source rows a set of LineRanges keeps, in
// source order, each row once. It is what skeleton output is rendered from:
// the rows remember where they came from, so slicing and numbering them can
// show real source positions.
type KeptLines struct {
	Lines [][]byte // the kept text of each row, without its newline
	Rows  []int    // the 1-based source row each line came from, ascending
}

// MergeRanges returns ranges sorted by Start, with the ones that overlap or
// touch joined, so no row is kept twice. A row kept whole by any range stays
// whole; one cut by every range that keeps it keeps the longest cut. A range
// that ends in a cut is not joined to one that starts on the next row: the
// cut row ends early, so the two are not one stretch of source.
func MergeRanges(ranges []LineRange) []LineRange {
	sorted := slices.Clone(ranges)
	slices.SortStableFunc(sorted, func(a, b LineRange) int { return a.Start - b.Start })

	var merged []LineRange
	for _, r := range sorted {
		if r.Start >= r.End {
			continue
		}
		n := len(merged) - 1
		if n < 0 || r.Start > merged[n].End || (r.Start == merged[n].End && merged[n].Cut > 0) {
			merged = append(merged, r)
			continue
		}
		cur := &merged[n]
		switch {
		case r.End > cur.End:
			cur.End, cur.Cut = r.End, r.Cut
		case r.End == cur.End && (cur.Cut == 0 || r.Cut == 0):
			cur.Cut = 0
		case r.End == cur.End:
			cur.Cut = max(cur.Cut, r.Cut)
		}
	}
	return merged
}

// KeepRanges collects the rows that ranges keep from src, merged as
// MergeRanges does, so ranges may come in any order and overlap.
func KeepRanges(src []byte, ranges []LineRange) KeptLines {
	lines := SplitLines(src)
	var k KeptLines
	for _, r := range MergeRanges(ranges) {
		end := min(r.End, len(lines))
		for row := max(r.Start, 0); row < end; row++ {
			text := lines[row]
			if row == r.End-1 && r.Cut > 0 && r.Cut < len(text) {
				text = text[:r.Cut]
			}
			k.Lines = append(k.Lines, text)
			k.Rows = append(k.Rows, row+1)
		}
	}
	return k
}

// Len is the number of kept lines.
func (k KeptLines) Len() int { return len(k.Lines) }

// Render returns the kept lines from..to-1 as text, each ending in a newline,
// with an ElisionMarker wherever two neighbouring lines are not neighbouring
// source rows. rows gives the source row of each output line, 0 for a marker,
// in the form LineNumberFormatter takes.
func (k KeptLines) Render(from, to int) (text []byte, rows []int) {
	from = max(from, 0)
	to = min(to, len(k.Lines))
	for i := from; i < to; i++ {
		if i > from && k.Rows[i] != k.Rows[i-1]+1 {
			text = append(text, ElisionMarker...)
			rows = append(rows, 0)
		}
		text = append(text, k.Lines[i]...)
		text = append(text, '\n')
		rows = append(rows, k.Rows[i])
	}
	return text, rows
}

// Text renders all the kept lines; see Render.
func (k KeptLines) Text() []byte {
	text, _ := k.Render(0, k.Len())
	return text
}
