package skeleton

import "testing"

// Per-language member tables are covered by the skeleton goldens in
// cmd/lx (testdata/languages/main.*, members.*); these tests pin the rules
// that cut across them.

func TestFunctionsOnly_LeavesOutTypesWithoutMethods(t *testing.T) {
	for _, tc := range []struct{ lang, src string }{
		{"java", "// Doc.\nclass Point {\n    public int x;\n}\n"},
		{"go", "package p\n\n// Doc.\ntype Point struct {\n\tX int\n}\n"},
		{"python", "# Doc.\nclass Point:\n    x = 1\n"},
		{"ruby", "# Doc.\nclass Point\n  attr_reader :x\nend\n"},
		{"rust", "/// Doc.\npub struct Point {\n    pub x: i32,\n}\n"},
		{"c", "/* Doc. */\ntypedef struct P {\n    int x;\n} P;\n"},
	} {
		if got := Extract(tc.lang, []byte(tc.src), true, false); len(got) != 0 {
			t.Errorf("%s: want nothing, got:\n%s", tc.lang, got)
		}
	}
}

func TestDocComments_PreprocessorIsNotDoc(t *testing.T) {
	for _, tc := range []struct{ lang, src, want string }{
		{"c", "#include <stdio.h>\n#define MAX 10\nint f(void);\n", "int f(void);\n"},
		{"cpp", "#include <vector>\n// Doc.\nint f();\n", "// Doc.\nint f();\n"},
		{"objectivec", "#import <Foundation/Foundation.h>\nint f(void) {\n  return 0;\n}\n", "int f(void) {\n"},
		{"csharp", "class A {\n    #region Api\n    public void F() {}\n}\n", "class A {\n...\n    public void F() {\n...\n}\n"},
	} {
		if got := string(Extract(tc.lang, []byte(tc.src), true, true)); got != tc.want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", tc.lang, got, tc.want)
		}
	}
}

func TestDocComments_PerLanguage(t *testing.T) {
	for _, tc := range []struct{ lang, src, want string }{
		// Rust attributes are siblings of their item and stay with it.
		{"rust", "#[inline]\npub fn f() {}\n", "#[inline]\npub fn f() {\n"},
		// PHP has # comments.
		{"php", "<?php\n# Doc.\nfunction f() {}\n", "# Doc.\nfunction f() {\n"},
		// A Python line starting with "//" is not a comment.
		{"python", "x = (1\n// 2)\ndef f():\n    pass\n", "def f():\n"},
	} {
		if got := string(Extract(tc.lang, []byte(tc.src), true, true)); got != tc.want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", tc.lang, got, tc.want)
		}
	}
}
