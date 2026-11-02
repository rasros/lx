package internal

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/fstest"
)

// refLines splits data into lines that keep their terminators.
func refLines(data []byte) [][]byte {
	var lines [][]byte
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			lines = append(lines, data)
			break
		}
		lines = append(lines, data[:i+1])
		data = data[i+1:]
	}
	return lines
}

func refHead(data []byte, n int) []byte {
	lines := refLines(data)
	if n < 0 || n > len(lines) {
		n = len(lines)
	}
	return bytes.Join(lines[:n], nil)
}

func refTail(data []byte, n int) []byte {
	lines := refLines(data)
	if n > len(lines) {
		n = len(lines)
	}
	return bytes.Join(lines[len(lines)-n:], nil)
}

// checkLineSource compares every read against the reference, each on a fresh
// source so the partial-read paths run, and then all on one shared source so
// the paths that reuse content already read run too.
func checkLineSource(t *testing.T, data []byte, counts []int) {
	t.Helper()
	fresh := func() *LineSource { return NewLineSource(bytes.NewReader(data), int64(len(data))) }
	shared := fresh()
	wantRows := len(refLines(data))

	for _, src := range []struct {
		name string
		get  func() *LineSource
	}{{"fresh", fresh}, {"shared", func() *LineSource { return shared }}} {
		for _, n := range counts {
			head, err := src.get().Head(n)
			if err != nil {
				t.Fatalf("%s Head(%d): %v", src.name, n, err)
			}
			if want := refHead(data, n); !bytes.Equal(head, want) {
				t.Errorf("%s Head(%d) = %q, want %q", src.name, n, abbrev(head), abbrev(want))
			}
			if n < 0 {
				continue
			}
			tail, err := src.get().Tail(n)
			if err != nil {
				t.Fatalf("%s Tail(%d): %v", src.name, n, err)
			}
			if want := refTail(data, n); !bytes.Equal(tail, want) {
				t.Errorf("%s Tail(%d) = %q, want %q", src.name, n, abbrev(tail), abbrev(want))
			}
		}
		rows, exact, err := src.get().Count()
		if err != nil {
			t.Fatalf("%s Count: %v", src.name, err)
		}
		if rows != wantRows || !exact {
			t.Errorf("%s Count() = %d, %v, want %d, true", src.name, rows, exact, wantRows)
		}
	}
}

func abbrev(b []byte) string {
	if len(b) > 80 {
		return fmt.Sprintf("%s...(%d bytes)", b[:80], len(b))
	}
	return string(b)
}

func TestLineSourceSmallContent(t *testing.T) {
	cases := map[string]string{
		"empty":               "",
		"single newline":      "\n",
		"blank lines":         "\n\n\n",
		"no trailing newline": "a\nb\nc",
		"one line no newline": "only",
		"trailing newline":    "a\nb\nc\n",
		"crlf":                "one\r\ntwo\r\nthree\r\n",
		"crlf no trailing":    "one\r\ntwo\r\nthree",
		"mixed endings":       "one\r\ntwo\nthree\r\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			checkLineSource(t, []byte(content), []int{-1, 0, 1, 2, 3, 4, 100})
		})
	}
}

func TestLineSourceKeepsCRLF(t *testing.T) {
	data := []byte("one\r\ntwo\r\nthree\r\nfour\r\n")
	src := NewLineSource(bytes.NewReader(data), int64(len(data)))
	head, _ := src.Head(2)
	tail, _ := src.Tail(2)
	if string(head) != "one\r\ntwo\r\n" {
		t.Errorf("Head(2) = %q, want CRLF kept", head)
	}
	if string(tail) != "three\r\nfour\r\n" {
		t.Errorf("Tail(2) = %q, want CRLF kept", tail)
	}
}

// TestLineSourceSpansChunks reads content several chunks long, with lines of
// varying length so line ends fall on and around chunk boundaries.
func TestLineSourceSpansChunks(t *testing.T) {
	var b bytes.Buffer
	for i := 0; b.Len() < 3*lineChunk+123; i++ {
		fmt.Fprintf(&b, "%d %s\n", i, strings.Repeat("x", i%257))
	}
	data := b.Bytes()
	rows := len(refLines(data))
	checkLineSource(t, data, []int{-1, 1, 2, 500, rows / 2, rows - 1, rows, rows + 1})
	// Without the final newline too.
	checkLineSource(t, data[:len(data)-1], []int{-1, 1, 500, rows - 1, rows, rows + 1})
}

func TestLineSourceLineLongerThanTenMegabytes(t *testing.T) {
	long := bytes.Repeat([]byte("x"), 11<<20)
	data := append(append(long, '\n'), "end\n"...)

	src := NewLineSource(bytes.NewReader(data), int64(len(data)))
	head, err := src.Head(1)
	if err != nil {
		t.Fatalf("Head(1): %v", err)
	}
	if len(head) != len(long)+1 || head[len(head)-1] != '\n' {
		t.Errorf("Head(1) has %d bytes, want the whole %d-byte line", len(head), len(long)+1)
	}

	src = NewLineSource(bytes.NewReader(data), int64(len(data)))
	tail, err := src.Tail(1)
	if err != nil {
		t.Fatalf("Tail(1): %v", err)
	}
	if string(tail) != "end\n" {
		t.Errorf("Tail(1) = %q, want %q", abbrev(tail), "end\n")
	}
	tail, err = src.Tail(2)
	if err != nil {
		t.Fatalf("Tail(2): %v", err)
	}
	if !bytes.Equal(tail, data) {
		t.Errorf("Tail(2) has %d bytes, want all %d", len(tail), len(data))
	}
	if rows, exact, _ := src.Count(); rows != 2 || !exact {
		t.Errorf("Count() = %d, %v, want 2, true", rows, exact)
	}
}

// TestLineSourceReadsAnyReaderAt uses a file from testing/fstest, which is
// neither an *os.File nor a *bytes.Reader.
func TestLineSourceReadsAnyReaderAt(t *testing.T) {
	content := "1\n2\n3\n4\n5\n"
	fsys := fstest.MapFS{"f.txt": {Data: []byte(content)}}
	f, err := fsys.Open("f.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ra, ok := f.(io.ReaderAt)
	if !ok {
		t.Fatalf("%T does not implement io.ReaderAt", f)
	}
	if _, isBytes := ra.(*bytes.Reader); isBytes {
		t.Fatal("want a reader that is not a *bytes.Reader")
	}

	src := NewLineSource(ra, int64(len(content)))
	head, err := src.Head(2)
	if err != nil || string(head) != "1\n2\n" {
		t.Errorf("Head(2) = %q, %v, want %q", head, err, "1\n2\n")
	}
	tail, err := src.Tail(2)
	if err != nil || string(tail) != "4\n5\n" {
		t.Errorf("Tail(2) = %q, %v, want %q", tail, err, "4\n5\n")
	}
	if rows, exact, err := src.Count(); rows != 5 || !exact || err != nil {
		t.Errorf("Count() = %d, %v, %v, want 5, true, nil", rows, exact, err)
	}
}

func TestLineSourceShrinksToShortContent(t *testing.T) {
	data := []byte("a\nb\n")
	src := NewLineSource(bytes.NewReader(data), 100)
	all, err := src.All()
	if err != nil || string(all) != "a\nb\n" {
		t.Fatalf("All() = %q, %v, want the content that is there", all, err)
	}
	if src.Size() != int64(len(data)) {
		t.Errorf("Size() = %d, want %d", src.Size(), len(data))
	}
}

// patternReader serves an endless repetition of a pattern, so a test can claim
// a huge size without allocating it.
type patternReader []byte

func (p patternReader) ReadAt(buf []byte, off int64) (int, error) {
	for i := range buf {
		buf[i] = p[(off+int64(i))%int64(len(p))]
	}
	return len(buf), nil
}

func TestLineSourceEstimatesBeyondExactCountLimit(t *testing.T) {
	pattern := patternReader("0123456789abcde\n") // 16 bytes per line
	size := ExactCountLimit + 16*1000
	src := NewLineSource(pattern, size)

	rows, exact, err := src.Count()
	if err != nil {
		t.Fatal(err)
	}
	if exact {
		t.Error("Count() reported exact for content beyond ExactCountLimit")
	}
	if want := int(size / 16); rows != want {
		t.Errorf("Count() = %d, want the estimate %d", rows, want)
	}

	tail, err := src.Tail(2)
	if err != nil || string(tail) != "0123456789abcde\n0123456789abcde\n" {
		t.Errorf("Tail(2) = %q, %v", tail, err)
	}
}

func TestLineNumberFormatterMarksApproximateTail(t *testing.T) {
	lnf := LineNumberFormatter{
		Head:        []byte("one\n"),
		Gap:         []byte("... (~7 rows skipped)\n"),
		Tail:        []byte("ten\n"),
		TotalRows:   10,
		Approximate: true,
	}
	// The "~" takes a column of its own, so head and tail numbers line up.
	want := "  1: one\n... (~7 rows skipped)\n~10: ten\n"
	if got := lnf.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// A file truncated in place between stat and read, as a log rotated with
// copytruncate is, holds less than the size the source was given. Reading its
// tail shrinks the source to the real end instead of failing.
func TestLineSourceTailOfContentShorterThanSize(t *testing.T) {
	var b bytes.Buffer
	for i := 0; b.Len() < 3*lineChunk; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	data := b.Bytes()
	wantTail := refTail(data, 2)

	for _, extra := range []int64{1, 100, lineChunk + 7, 10 * lineChunk} {
		declared := int64(len(data)) + extra

		t.Run(fmt.Sprintf("+%d tail only", extra), func(t *testing.T) {
			src := NewLineSource(bytes.NewReader(data), declared)
			tail, err := src.Tail(2)
			if err != nil {
				t.Fatalf("Tail(2): %v", err)
			}
			if !bytes.Equal(tail, wantTail) {
				t.Errorf("Tail(2) = %q, want %q", tail, wantTail)
			}
			if src.Size() != int64(len(data)) {
				t.Errorf("Size() = %d, want %d", src.Size(), len(data))
			}
		})

		t.Run(fmt.Sprintf("+%d head then tail", extra), func(t *testing.T) {
			src := NewLineSource(bytes.NewReader(data), declared)
			head, err := src.Head(2)
			if err != nil || !bytes.Equal(head, refHead(data, 2)) {
				t.Fatalf("Head(2) = %q, %v", head, err)
			}
			tail, err := src.Tail(2)
			if err != nil {
				t.Fatalf("Tail(2): %v", err)
			}
			if !bytes.Equal(tail, wantTail) {
				t.Errorf("Tail(2) = %q, want %q", tail, wantTail)
			}
			rows, exact, err := src.Count()
			if err != nil || !exact || rows != len(refLines(data)) {
				t.Errorf("Count() = %d, %v, %v, want %d, true, nil", rows, exact, err, len(refLines(data)))
			}
		})
	}
}

func TestLineNumberFormatterAlignsApproximateTailWithHead(t *testing.T) {
	lnf := LineNumberFormatter{
		Head:        []byte("a\nb\n"),
		Gap:         []byte("... (~45249996 rows skipped)\n"),
		Tail:        []byte("y\nz\n"),
		TotalRows:   45249999,
		Approximate: true,
	}
	lines := strings.Split(strings.TrimSuffix(lnf.String(), "\n"), "\n")
	want := []string{
		"        1: a",
		"        2: b",
		"... (~45249996 rows skipped)",
		"~45249998: y",
		"~45249999: z",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}
