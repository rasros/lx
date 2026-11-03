package skeleton

import (
	"testing"

	"github.com/rasros/lx/pkg/lx/internal"
)

func TestCppEvidence(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"enum class", "enum class Color { Red, Green };\n", true},
		{"method", "struct P { int f() const; };\n", true},
		{"reference", "int f(int &r);\n", true},
		{"digit separator", "int k = 1'000;\n", true},
		{"asm clobbers", "static inline void b(void) { asm volatile(\"\" ::: \"memory\"); }\n", false},
		{"attribute", "[[gnu::always_inline]] int h(void);\n", false},
		{"template call in a body", "int f(void) { return a < b > (c); }\n", false},
		{"function pointer field", "struct ops { int (*f)(void); };\n", false},
		{"identifiers new and class", "struct page *new;\nint class;\n", false},
	}
	for _, tt := range tests {
		p, ok := parse("cpp", []byte(tt.src))
		if !ok {
			t.Fatalf("%s: no parse", tt.name)
		}
		if got := cppEvidence(p, []byte(tt.src)); got != tt.want {
			t.Errorf("%s: cppEvidence = %v, want %v", tt.name, got, tt.want)
		}
		p.tree.Release()
	}
}

func TestCppClean(t *testing.T) {
	tests := []struct {
		name, src string
		want      bool
	}{
		{"clean C++", "struct P { int f() const; };\n", true},
		{"clean C", "int f(void);\n", false},
		{"C++ with errors", "class C { int f(); };\nstd::vector<std::pair<int, int> > g(int x);\n", false},
	}
	for _, tt := range tests {
		p, ok := parse("cpp", []byte(tt.src))
		if !ok {
			t.Fatalf("%s: no parse", tt.name)
		}
		if got := cppClean(p, []byte(tt.src)); got != tt.want {
			t.Errorf("%s: cppClean = %v, want %v", tt.name, got, tt.want)
		}
		p.tree.Release()
	}
}

func TestTrailingComments(t *testing.T) {
	src := []byte("struct s {\n  int a;  /* x,\n     y */\n  /* doc\n   */\n  int b; /* one row */\n};\n")
	p, _ := parse("c", src)
	defer p.tree.Release()
	trailing := p.trailingComments(internal.SplitLines(src))
	if len(trailing) != 1 || trailing[1] != 8 {
		t.Errorf("trailing = %v, want map[1:8]", trailing)
	}
}
