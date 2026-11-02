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
	got := Ranges(lang, []byte(src), functions, structs)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Ranges = %v\nwant     %v", got, want)
	}
	if text := string(Extract(lang, []byte(src), functions, structs)); text != wantText {
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
	k := Keep("go", []byte(goRangesSrc), true, false)
	if want := []int{18, 19, 21}; !reflect.DeepEqual(k.Rows, want) {
		t.Errorf("rows = %v, want %v", k.Rows, want)
	}
}

const pyRangesSrc = `import os


# Shape doc
@dataclass
class Shape:
    """A shape."""

    name: str = ""

    @property
    def area(self) -> float:
        """Area."""
        return 0.0

    def _hidden(self):
        pass

    class Inner:
        def go(self):
            pass


@cache
def top(x):
    return x
`

func TestRanges_Python(t *testing.T) {
	checkRanges(t, "python", pyRangesSrc, true, true, []Range{
		{Start: 3, End: 7}, // comment, decorator, class, docstring
		{Start: 8, End: 9},
		{Start: 10, End: 13}, // decorator, signature, docstring
		{Start: 18, End: 20}, // nested class and its method
		{Start: 23, End: 25},
	}, `# Shape doc
@dataclass
class Shape:
    """A shape."""
...
    name: str = ""
...
    @property
    def area(self) -> float:
        """Area."""
...
    class Inner:
        def go(self):
...
@cache
def top(x):
`)
}

const tsRangesSrc = `import x from "y";

/** Shape doc. */
export interface Shape {
  area(): number;
}

export class Box {
  /** Width. */
  width: number = 0;
  private secret = 1;
  @Input()
  size(): number {
    return 1;
  }
}

export const add = (a: number, b: number): number => {
  return a + b;
};
`

func TestRanges_TypeScript(t *testing.T) {
	checkRanges(t, "typescript", tsRangesSrc, true, true, []Range{
		{Start: 2, End: 6},   // doc, interface, member, closing
		{Start: 7, End: 10},  // class, field doc, field
		{Start: 11, End: 13}, // decorator, method signature
		{Start: 15, End: 16},
		{Start: 17, End: 18},
	}, `/** Shape doc. */
export interface Shape {
  area(): number;
}
...
export class Box {
  /** Width. */
  width: number = 0;
...
  @Input()
  size(): number {
...
}
...
export const add = (a: number, b: number): number => {
`)
}

const rustRangesSrc = `use std::fmt;

/// A point.
#[derive(Debug)]
pub struct Point {
    /// X coord.
    pub x: f64,
    y: f64,
}

impl Point {
    /// New point.
    pub fn new(x: f64) -> Self { Point { x, y: 0.0 } }

    fn private(&self) {}
}

pub trait Area {
    fn area(&self) -> f64;
}

pub fn free() -> i32 {
    1
}
`

func TestRanges_Rust(t *testing.T) {
	checkRanges(t, "rust", rustRangesSrc, true, true, []Range{
		{Start: 2, End: 7}, // doc, attribute, struct, field doc, field
		{Start: 8, End: 9},
		{Start: 10, End: 13, Cut: cutAfterBrace(t, rustRangesSrc, 12)},
		{Start: 15, End: 16},
		{Start: 17, End: 20},
		{Start: 21, End: 22},
	}, `/// A point.
#[derive(Debug)]
pub struct Point {
    /// X coord.
    pub x: f64,
...
}
...
impl Point {
    /// New point.
    pub fn new(x: f64) -> Self {
...
}
...
pub trait Area {
    fn area(&self) -> f64;
}
...
pub fn free() -> i32 {
`)
}

func TestRanges_KeepsEverythingWhenNotReduced(t *testing.T) {
	src := []byte("a\nb\nc")
	want := []Range{{Start: 0, End: 3}}
	if got := Ranges("erlang", src, true, true); !reflect.DeepEqual(got, want) {
		t.Errorf("unsupported: %v, want %v", got, want)
	}
	if got := Ranges("go", src, false, false); !reflect.DeepEqual(got, want) {
		t.Errorf("no flags: %v, want %v", got, want)
	}
	if got := Ranges("go", nil, true, true); got != nil {
		t.Errorf("empty source: %v", got)
	}
}
