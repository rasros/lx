package skeleton

import (
	"bytes"
	"reflect"
	"testing"
)

// cutAfterBrace is the Cut that keeps row of src up to and including its
// first "{".
func cutAfterBrace(t *testing.T, src string, row int) int {
	t.Helper()
	line := bytes.Split([]byte(src), []byte("\n"))[row]
	i := bytes.IndexByte(line, '{')
	if i < 0 {
		t.Fatalf("row %d has no brace: %q", row, line)
	}
	return i + 1
}

func checkRanges(t *testing.T, lang, src string, functions, structs bool, want []Range, wantText string) {
	t.Helper()
	got := Ranges(lang, false, []byte(src), functions, structs)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Ranges = %v\nwant     %v", got, want)
	}
	if text := string(Extract(lang, false, []byte(src), functions, structs)); text != wantText {
		t.Errorf("Extract =\n%s\nwant\n%s", text, wantText)
	}
}

const goRangesSrc = `package geo

// Point is a point.
type Point struct {
	// X is horizontal.
	X float64
	y float64
}

type (
	// Inner is nested in a group.
	Inner struct {
		A int
	}
	hidden int
)

// Add adds.
func Add(a, b int) int { return a + b }

func (p Point) Len() float64 {
	return p.X
}
`

func TestRanges_Go(t *testing.T) {
	checkRanges(t, "go", goRangesSrc, true, true, []Range{
		{Start: 2, End: 6}, // doc, header, field doc, field
		{Start: 7, End: 8}, // closing brace after the unexported field
		{Start: 9, End: 14},
		{Start: 15, End: 16},
		{Start: 17, End: 19, Cut: cutAfterBrace(t, goRangesSrc, 18)},
		{Start: 20, End: 21},
	}, `// Point is a point.
type Point struct {
	// X is horizontal.
	X float64
...
}
...
type (
	// Inner is nested in a group.
	Inner struct {
		A int
	}
...
)
...
// Add adds.
func Add(a, b int) int {
...
func (p Point) Len() float64 {
`)
}

func TestKeep_GoRows(t *testing.T) {
	k := Keep("go", false, []byte(goRangesSrc), true, false)
	if want := []int{18, 19, 21}; !reflect.DeepEqual(k.Rows, want) {
		t.Errorf("rows = %v, want %v", k.Rows, want)
	}
}

func TestRanges_KeepsEverythingWhenNotReduced(t *testing.T) {
	src := []byte("a\nb\nc")
	want := []Range{{Start: 0, End: 3}}
	if got := Ranges("erlang", false, src, true, true); !reflect.DeepEqual(got, want) {
		t.Errorf("unsupported: %v, want %v", got, want)
	}
	if got := Ranges("go", false, src, false, false); !reflect.DeepEqual(got, want) {
		t.Errorf("no flags: %v, want %v", got, want)
	}
	if got := Ranges("go", false, nil, true, true); got != nil {
		t.Errorf("empty source: %v", got)
	}
}

func TestExtract_PassesUnreducedSourceThrough(t *testing.T) {
	// No final newline: Extract must not add one to source it leaves alone.
	src := []byte("a\nb\nc")
	for _, tc := range []struct {
		name               string
		lang               string
		functions, structs bool
	}{
		{"unsupported", "erlang", true, true},
		{"no flags", "go", false, false},
	} {
		if got := Extract(tc.lang, false, src, tc.functions, tc.structs); string(got) != string(src) {
			t.Errorf("%s: Extract = %q, want %q unchanged", tc.name, got, src)
		}
	}
	if got := Extract("go", false, nil, true, true); len(got) != 0 {
		t.Errorf("empty source: %q", got)
	}
}

func TestExtract_MarksInlineCuts(t *testing.T) {
	src := "def f(): return 1\ndef g(): return 2\ndef h(): return 3\n"
	want := "def f(): \n...\ndef g(): \n...\ndef h(): \n"
	if got := string(Extract("python", false, []byte(src), true, false)); got != want {
		t.Errorf("Extract = %q, want %q", got, want)
	}
	k := Keep("python", false, []byte(src), true, false)
	if want := []int{1, 2, 3}; !reflect.DeepEqual(k.Rows, want) {
		t.Errorf("rows = %v, want %v", k.Rows, want)
	}
}
