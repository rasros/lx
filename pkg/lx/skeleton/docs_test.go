package skeleton

import (
	"strings"
	"testing"
)

func TestC_MultiLineBlockComment(t *testing.T) {
	src := []byte(`/* Subtract b from a
   across two lines. */
int subtract(int a, int b);
`)
	out := string(Extract("c", false, src, true, false))
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
	out := string(Extract("c", false, src, false, true))
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

func TestDocComments_BlankLineSeparates(t *testing.T) {
	// A comment block separated from a decl by a blank line is NOT a doc comment.
	src := []byte(`package p

// Top-of-file comment, not attached to anything.

func Helper() {}

// Real doc for Real.
func Real() {}
`)
	out := string(Extract("go", false, src, true, false))
	if strings.Contains(out, "Top-of-file comment") {
		t.Errorf("orphan comment should not attach to Helper:\n%s", out)
	}
	if !strings.Contains(out, "// Real doc for Real.") {
		t.Errorf("expected doc for Real, got:\n%s", out)
	}
}
