package langs

import (
	"testing"

	"github.com/rasros/lx/pkg/lx/internal"
)

func TestLeadingDocStart(t *testing.T) {
	tests := []struct {
		name string
		cs   *CommentSyntax
		src  string // the declaration is the last line
		want int    // first doc row, 0-based; the last row for none
	}{
		{"unstarred block with code-like lines", &CComments, "int a;\n/*\n   Example: x = 1;\n   #define y\n*/\nint f(void);", 1},
		{"starred block", &CComments, "/**\n * f.\n */\nint f(void);", 0},
		{"doc line holding comment markers", &CComments, "/**\n * a `/* x */` b\n */\nint f(void);", 0},
		{"line comment ending in a closer", &CComments, "/* h */\nint a;\n// globs **/\nint f(void);", 2},
		{"trailing field comment", &CComments, "  int a; /* x,\n   * more */\n  int b;", 2},
		{"stray closer", &CComments, "int a;\n   done */\nint f(void);", 2},
		{"closer after another comment ends", &CComments, "/* a */\n   b */\nint f(void);", 2},
		{"haskell block with code-like text", &HaskellComments, "{- a\n   let x = 1; -}\nf = 1", 0},
	}
	for _, tt := range tests {
		lines := internal.SplitLines([]byte(tt.src))
		row := len(lines) - 1
		if got := tt.cs.LeadingDocStart(lines, row); got != tt.want {
			t.Errorf("%s: LeadingDocStart = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestOpenerAbove(t *testing.T) {
	b := CComments.Blocks[0]
	tests := []struct {
		name  string
		above string // the lines above a closing line
		want  int
	}{
		{"opener on its own line", "/*\n text;", 0},
		{"nearest opener wins", "/* a\n */\n/*\n b", 2},
		{"opener after code", "int a; /* x,\n * y", -1},
		{"another comment closes first", "/* a */\n b", -1},
		{"none", "int a;\n b", -1},
	}
	for _, tt := range tests {
		lines := internal.SplitLines([]byte(tt.above))
		if got := CComments.openerAbove(b, lines, len(lines)-1); got != tt.want {
			t.Errorf("%s: openerAbove = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestLineComment(t *testing.T) {
	for line, want := range map[string]bool{
		"  // globs **/": true,
		"/* x */":        false,
		" * more */":     false,
		"int a; // b */": false,
	} {
		if got := CComments.lineComment([]byte(line)); got != want {
			t.Errorf("lineComment(%q) = %v, want %v", line, got, want)
		}
	}
	if !HaskellComments.lineComment([]byte("-- x -}")) {
		t.Errorf("a Haskell -- line is a line comment")
	}
}
