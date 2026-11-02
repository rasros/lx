package internal

import (
	"reflect"
	"testing"
)

func TestMergeRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []LineRange
		want []LineRange
	}{
		{"empty", nil, nil},
		{"drops empty ranges", []LineRange{{Start: 2, End: 2}, {Start: 4, End: 3}}, nil},
		{"sorts", []LineRange{{Start: 5, End: 6}, {Start: 1, End: 2}},
			[]LineRange{{Start: 1, End: 2}, {Start: 5, End: 6}}},
		{"joins overlapping", []LineRange{{Start: 1, End: 4}, {Start: 3, End: 6}},
			[]LineRange{{Start: 1, End: 6}}},
		{"joins adjacent", []LineRange{{Start: 1, End: 3}, {Start: 3, End: 5}},
			[]LineRange{{Start: 1, End: 5}}},
		{"keeps a gap", []LineRange{{Start: 1, End: 3}, {Start: 4, End: 5}},
			[]LineRange{{Start: 1, End: 3}, {Start: 4, End: 5}}},
		{"swallows contained", []LineRange{{Start: 1, End: 9}, {Start: 3, End: 4, Cut: 2}},
			[]LineRange{{Start: 1, End: 9}}},
		{"whole row beats cut", []LineRange{{Start: 2, End: 3, Cut: 4}, {Start: 2, End: 3}},
			[]LineRange{{Start: 2, End: 3}}},
		{"longest of two cuts", []LineRange{{Start: 2, End: 3, Cut: 4}, {Start: 1, End: 3, Cut: 7}},
			[]LineRange{{Start: 1, End: 3, Cut: 7}}},
		{"later range uncuts a row", []LineRange{{Start: 1, End: 3, Cut: 4}, {Start: 2, End: 5}},
			[]LineRange{{Start: 1, End: 5}}},
		{"later range brings its cut", []LineRange{{Start: 1, End: 3}, {Start: 2, End: 5, Cut: 3}},
			[]LineRange{{Start: 1, End: 5, Cut: 3}}},
		{"cut row is not joined to the next", []LineRange{{Start: 1, End: 3, Cut: 4}, {Start: 3, End: 5}},
			[]LineRange{{Start: 1, End: 3, Cut: 4}, {Start: 3, End: 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MergeRanges(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("MergeRanges(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMergeRangesLeavesInputAlone(t *testing.T) {
	in := []LineRange{{Start: 5, End: 6}, {Start: 1, End: 2}}
	MergeRanges(in)
	if in[0].Start != 5 {
		t.Errorf("input reordered: %v", in)
	}
}

func TestKeepLinesClips(t *testing.T) {
	lines := SplitLines([]byte("a\nb\nc"))
	got := KeepLines(nil, lines, -2, 10)
	got = KeepLines(got, lines, 2, 1) // empty
	got = KeepLine(got, lines, 7)     // out of range
	if want := []LineRange{{Start: 0, End: 3}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestKeepPrefix(t *testing.T) {
	lines := SplitLines([]byte("func f() { return 1 }\n"))
	if got := KeepPrefix(nil, lines, 0, 10); !reflect.DeepEqual(got, []LineRange{{Start: 0, End: 1, Cut: 10}}) {
		t.Errorf("got %v", got)
	}
	// A cut at or past the end keeps the whole line.
	if got := KeepPrefix(nil, lines, 0, 99); !reflect.DeepEqual(got, []LineRange{{Start: 0, End: 1}}) {
		t.Errorf("got %v", got)
	}
	if got := KeepPrefix(nil, lines, 5, 3); got != nil {
		t.Errorf("out-of-range row kept: %v", got)
	}
}

const fiveRows = "one\ntwo\nthree\nfour\nfive\n"

func TestKeepRanges(t *testing.T) {
	k := KeepRanges(SplitLines([]byte(fiveRows)), []LineRange{
		{Start: 3, End: 5},
		{Start: 0, End: 1},
		{Start: 1, End: 2, Cut: 2}, // touches row 0, so no gap
		{Start: 4, End: 5},         // overlaps
	})
	wantLines := []string{"one", "tw", "four", "five"}
	var gotLines []string
	for _, l := range k.Lines {
		gotLines = append(gotLines, string(l))
	}
	if !reflect.DeepEqual(gotLines, wantLines) {
		t.Errorf("lines = %q, want %q", gotLines, wantLines)
	}
	if want := []int{1, 2, 4, 5}; !reflect.DeepEqual(k.Rows, want) {
		t.Errorf("rows = %v, want %v", k.Rows, want)
	}
}

func TestKeptLinesRender(t *testing.T) {
	k := KeepRanges(SplitLines([]byte(fiveRows)), []LineRange{{Start: 0, End: 2}, {Start: 3, End: 4}, {Start: 4, End: 5}})

	text, rows := k.Render(0, k.Len())
	if want := "one\ntwo\n...\nfour\nfive\n"; string(text) != want {
		t.Errorf("text = %q, want %q", text, want)
	}
	if want := []int{1, 2, 0, 4, 5}; !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
	if got := string(k.Text()); got != string(text) {
		t.Errorf("Text() = %q, want %q", got, text)
	}

	// A slice has markers only between its own lines, never at its edges.
	text, rows = k.Render(1, 3)
	if want := "two\n...\nfour\n"; string(text) != want {
		t.Errorf("slice text = %q, want %q", text, want)
	}
	if want := []int{2, 0, 4}; !reflect.DeepEqual(rows, want) {
		t.Errorf("slice rows = %v, want %v", rows, want)
	}
	if text, rows = k.Render(3, 3); text != nil || rows != nil {
		t.Errorf("empty slice = %q %v", text, rows)
	}
}

func TestKeptLinesMarkCutLines(t *testing.T) {
	// A cut leaves out the rest of its row, so it is marked like left-out
	// rows are, even though the next kept line is the next row.
	src := "def f(): return 1\ndef g(): return 2\nclass A: pass\n\ndef h(): return 3\n"
	k := KeepRanges(SplitLines([]byte(src)), []LineRange{
		{Start: 0, End: 1, Cut: 9},
		{Start: 1, End: 2, Cut: 9},
		{Start: 2, End: 3},
		{Start: 4, End: 5, Cut: 9},
	})
	text, rows := k.Render(0, k.Len())
	// One marker after each cut and one for the blank row 4, never two in a
	// row, and none after the last line though it is cut too.
	want := "def f(): \n...\ndef g(): \n...\nclass A: pass\n...\ndef h(): \n"
	if string(text) != want {
		t.Errorf("text = %q, want %q", text, want)
	}
	if wantRows := []int{1, 0, 2, 0, 3, 0, 5}; !reflect.DeepEqual(rows, wantRows) {
		t.Errorf("rows = %v, want %v", rows, wantRows)
	}
	if want := []bool{true, true, false, true}; !reflect.DeepEqual(k.Cut, want) {
		t.Errorf("cut = %v, want %v", k.Cut, want)
	}
	// A cut row and a gap after it still make one marker.
	text, _ = k.Render(1, 4)
	if want := "def g(): \n...\nclass A: pass\n...\ndef h(): \n"; string(text) != want {
		t.Errorf("slice text = %q, want %q", text, want)
	}
}

func TestKeptLinesSize(t *testing.T) {
	src := SplitLines([]byte("def f(): return 1\nx\ny\n\nz\n"))
	for _, ranges := range [][]LineRange{
		nil,
		{{Start: 0, End: 1}},
		{{Start: 0, End: 1, Cut: 9}, {Start: 1, End: 3}},
		{{Start: 0, End: 2}, {Start: 4, End: 5}},
	} {
		k := KeepRanges(src, ranges)
		if got, want := k.Size(), len(k.Text()); got != want {
			t.Errorf("%v: Size() = %d, Text() is %d bytes", ranges, got, want)
		}
	}
}
