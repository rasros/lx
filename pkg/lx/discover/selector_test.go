package discover

import (
	"path/filepath"
	"testing"
)

func TestSelector(t *testing.T) {
	out, err := filepath.Abs("bundle.md")
	if err != nil {
		t.Fatal(err)
	}
	s := newSelector(Options{Includes: []string{"src/*.go", "*.md"}, Excludes: []string{"*_test.go"}, OutPath: out})

	for _, tc := range []struct {
		name string
		got  bool
		want bool
	}{
		{"piped file matching an include", s.AcceptRoot("src/main.go", false), true},
		{"piped file matching an exclude", s.AcceptRoot("src/main_test.go", false), false},
		{"piped file matching no include", s.AcceptRoot("lib/util.go", false), false},
		{"piped dir an include reaches into", s.AcceptRoot("src", true), true},
		{"descend into an include's prefix", s.Descend("src"), true},
		{"descend where a floating include could match", s.Descend("docs"), true},
		{"walked file matching an include", s.Accept("src/main.go", "src/main.go"), true},
		{"walked file matching no include", s.Accept("lib/util.go", "lib/util.go"), false},
		{"the output file", s.Accept("bundle.md", "bundle.md"), false},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	anchored := newSelector(Options{Includes: []string{"src/*.go"}})
	if anchored.Descend("lib") || anchored.AcceptRoot("lib", true) {
		t.Error("kept lib, which no include can match below")
	}
	if all := newSelector(Options{}); !all.Descend("any") || !all.Accept("any/file", "any/file") {
		t.Error("a selector without includes rejected a path")
	}
}
