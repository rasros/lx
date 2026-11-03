package langs

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/rasros/lx/pkg/lx/internal"
)

// parse returns a Ctx for src and its syntax tree's root.
func parse(t *testing.T, lang *gotreesitter.Language, src string) (*Ctx, *gotreesitter.Node) {
	t.Helper()
	tree, err := gotreesitter.NewParserPool(lang).Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tree.Release)
	c := &Ctx{Src: []byte(src), Lines: internal.SplitLines([]byte(src)), Lang: lang, Comments: &CComments}
	return c, tree.RootNode()
}

// members returns the named children of the body of the first node of type
// typ under n, the node itself, in source order.
func members(t *testing.T, c *Ctx, n *gotreesitter.Node, typ string, bodyTypes ...string) (*gotreesitter.Node, []*gotreesitter.Node) {
	t.Helper()
	decl := find(c, n, typ)
	if decl == nil {
		t.Fatalf("no %s", typ)
	}
	body := (&Members{BodyField: "body", Bodies: bodyTypes}).body(c, decl)
	if body == nil {
		t.Fatalf("%s has no body", typ)
	}
	var out []*gotreesitter.Node
	for i := 0; i < body.ChildCount(); i++ {
		if ch := body.Child(i); ch.IsNamed() && c.Type(ch) != "comment" {
			out = append(out, ch)
		}
	}
	return decl, out
}

func find(c *Ctx, n *gotreesitter.Node, typ string) *gotreesitter.Node {
	if c.Type(n) == typ {
		return n
	}
	for i := 0; i < n.ChildCount(); i++ {
		if f := find(c, n.Child(i), typ); f != nil {
			return f
		}
	}
	return nil
}

func TestCppMember(t *testing.T) {
	c, root := parse(t, grammars.CppLanguage(), `class C {
public:
    struct Inner { int a; };
    struct { int b; } anon;
    void (*callback)(int);
    int *make();
    int &ref();
    int plain;
    void decl();
    void def() {}
    C();
    template <typename T> T get(T x) { return x; }
    enum class E { A };
    operator int() const;
    friend bool operator==(const C& a, const C& b);
    friend class F;
};
`)
	_, ms := members(t, c, root, "class_specifier")
	want := []struct {
		text string
		kind MemberKind
	}{
		{"public", Skip}, // the access_specifier label
		{"struct Inner { int a; };", Nested},
		{"struct { int b; } anon;", Field},
		{"void (*callback)(int);", Field},
		{"int *make();", Method},
		{"int &ref();", Method},
		{"int plain;", Field},
		{"void decl();", Method},
		{"void def() {}", Method},
		{"C();", Method},
		{"template <typename T> T get(T x) { return x; }", Method},
		{"enum class E { A };", Nested},
		{"operator int() const;", Method},
		{"friend bool operator==(const C& a, const C& b);", Method},
		{"friend class F;", Field},
	}
	if len(ms) != len(want) {
		for _, m := range ms {
			t.Logf("%s %q", c.Type(m), c.Text(m))
		}
		t.Fatalf("got %d members, want %d", len(ms), len(want))
	}
	for i, w := range want {
		m := ms[i]
		if got := c.Text(m); got != w.text {
			t.Fatalf("member %d is %q, want %q", i, got, w.text)
		}
		if got := CppMember(c, CppUnwrap(c, m)); got != w.kind {
			t.Errorf("%q: kind %d, want %d", w.text, got, w.kind)
		}
	}
}
