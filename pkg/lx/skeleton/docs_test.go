package skeleton

import (
	"strings"
	"testing"
)

func TestGo_DocComments(t *testing.T) {
	src := []byte(`package p

// Point is a 2D point.
type Point struct {
	// X is the horizontal coordinate.
	X float64
	Y float64
}

/* NewPoint creates a Point. */
func NewPoint(x, y float64) Point {
	return Point{x, y}
}
`)
	out := string(Extract("go", src, true, true))
	for _, want := range []string{
		"// Point is a 2D point.",
		"\t// X is the horizontal coordinate.",
		"/* NewPoint creates a Point. */",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPython_Docstrings(t *testing.T) {
	src := []byte(`# leading comment for animal
class Animal:
    """Base class for all animals."""

    species = "unknown"

    def speak(self):
        """Return the animal's sound."""
        return "..."

# helper for create
def create(name):
    """Create something."""
    return name
`)
	out := string(Extract("python", src, true, true))
	for _, want := range []string{
		"# leading comment for animal",
		`    """Base class for all animals."""`,
		`        """Return the animal's sound."""`,
		"# helper for create",
		`    """Create something."""`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRust_DocComments(t *testing.T) {
	src := []byte(`/// User type.
pub struct User {
    /// Public name.
    pub name: String,
}

/// Create a user.
pub fn make(name: String) -> User { User { name } }
`)
	out := string(Extract("rust", src, true, true))
	for _, want := range []string{
		"/// User type.",
		"    /// Public name.",
		"/// Create a user.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestJava_Javadoc(t *testing.T) {
	src := []byte(`/**
 * The Calculator.
 */
public class Calculator {
    /** Initial value. */
    public int value;

    /**
     * Add x to value.
     */
    public int add(int x) { return value + x; }
}
`)
	out := string(Extract("java", src, true, true))
	for _, want := range []string{
		"/**",
		" * The Calculator.",
		"    /** Initial value. */",
		"     * Add x to value.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTypeScript_DocComments(t *testing.T) {
	src := []byte(`/** A point. */
export class Point {
    /** Horizontal axis. */
    public x: number = 0;

    /** Scale by factor. */
    public scale(factor: number): number { return this.x * factor; }
}

/** Top-level fn. */
export function topLevel(x: number): number { return x; }
`)
	out := string(Extract("typescript", src, true, true))
	for _, want := range []string{
		"/** A point. */",
		"    /** Horizontal axis. */",
		"    /** Scale by factor. */",
		"/** Top-level fn. */",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestC_MultiLineBlockComment(t *testing.T) {
	src := []byte(`/* Subtract b from a
   across two lines. */
int subtract(int a, int b);
`)
	out := string(Extract("c", src, true, false))
	for _, want := range []string{
		"/* Subtract b from a",
		"   across two lines. */",
		"int subtract(int a, int b);",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestC_InlineCommentNotMultiLineCloser(t *testing.T) {
	// Self-contained `/* x */` on a field line must not pull a stray
	// preceding comment into the next field.
	src := []byte(`/* leading */
typedef struct P {
    int x; /* per-field */
    int y;
} P;
`)
	out := string(Extract("c", src, false, true))
	if strings.Count(out, "/* leading */") != 1 {
		t.Errorf("leading comment duplicated:\n%s", out)
	}
	if strings.Count(out, "typedef struct P") != 1 {
		t.Errorf("struct header duplicated:\n%s", out)
	}
	if !strings.Contains(out, "    int y;") {
		t.Errorf("missing y field:\n%s", out)
	}
}

func TestHaskell_BlockComment(t *testing.T) {
	src := []byte(`module M where

{- A newtype wrapper.
   With a continuation. -}
newtype Name = Name String
`)
	out := string(Extract("haskell", src, false, true))
	for _, want := range []string{
		"{- A newtype wrapper.",
		"   With a continuation. -}",
		"newtype Name = Name String",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestOCaml_BlockCommentAndModuleMember(t *testing.T) {
	src := []byte(`(* Outer doc. *)
module M = struct
  (* Inner doc. *)
  let pi = 3.14

  (* Multi-line
     block comment. *)
  let area r =
    pi *. r *. r
end
`)
	out := string(Extract("ocaml", src, true, true))
	for _, want := range []string{
		"(* Outer doc. *)",
		"  (* Inner doc. *)",
		"  let pi = 3.14",
		"  (* Multi-line",
		"     block comment. *)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRuby_BeginEndBlock(t *testing.T) {
	src := []byte(`=begin
Doc for combine.
=end
def combine(x, y)
  x + y
end
`)
	out := string(Extract("ruby", src, true, false))
	for _, want := range []string{
		"=begin",
		"Doc for combine.",
		"=end",
		"def combine(x, y)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDocComments_BlankLineSeparates(t *testing.T) {
	// A comment block separated from a decl by a blank line is NOT a doc comment.
	src := []byte(`package p

// Top-of-file comment, not attached to anything.

func Helper() {}

// Real doc for Real.
func Real() {}
`)
	out := string(Extract("go", src, true, false))
	if strings.Contains(out, "Top-of-file comment") {
		t.Errorf("orphan comment should not attach to Helper:\n%s", out)
	}
	if !strings.Contains(out, "// Real doc for Real.") {
		t.Errorf("expected doc for Real, got:\n%s", out)
	}
}
