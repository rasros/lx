package walker

import (
	"reflect"
	"testing"
)

func TestCompileSpecMetadata(t *testing.T) {
	tests := []struct {
		name              string
		raw               string
		wantPattern       string
		wantDirOnly       bool
		wantAnchored      bool
		wantLiteral       bool
		wantHasSlash      bool
		wantHasDoubleStar bool
		wantOnlyStar      bool
		wantPrefix        string
		wantSuffix        string
		wantValid         bool
	}{
		{
			name:         "literal",
			raw:          "main.go",
			wantPattern:  "main.go",
			wantLiteral:  true,
			wantValid:    true,
			wantOnlyStar: false,
		},
		{
			name:         "star suffix glob",
			raw:          "*.go",
			wantPattern:  "*.go",
			wantLiteral:  false,
			wantValid:    true,
			wantOnlyStar: true,
			wantPrefix:   "",
			wantSuffix:   ".go",
		},
		{
			name:         "star infix glob",
			raw:          "foo*bar",
			wantPattern:  "foo*bar",
			wantLiteral:  false,
			wantValid:    true,
			wantOnlyStar: true,
			wantPrefix:   "foo",
			wantSuffix:   "bar",
		},
		{
			name:              "double star path glob",
			raw:               "**/*.go",
			wantPattern:       "**/*.go",
			wantLiteral:       false,
			wantValid:         true,
			wantHasSlash:      true,
			wantHasDoubleStar: true,
			wantOnlyStar:      false,
		},
		{
			name:         "anchored and dir only",
			raw:          "/vendor/",
			wantPattern:  "vendor",
			wantDirOnly:  true,
			wantAnchored: true,
			wantLiteral:  true,
			wantValid:    true,
		},
		{
			name:         "windows slashes normalized",
			raw:          `\src\*.go`,
			wantPattern:  "src/*.go",
			wantAnchored: true,
			wantLiteral:  false,
			wantValid:    true,
			wantHasSlash: true,
			wantOnlyStar: true,
			wantPrefix:   "src/",
			wantSuffix:   ".go",
		},
		{
			name:         "invalid glob",
			raw:          "[",
			wantPattern:  "[",
			wantLiteral:  false,
			wantValid:    false,
			wantOnlyStar: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := compileSpec(tt.raw)
			if spec.Pattern.Pattern != tt.wantPattern {
				t.Fatalf("Pattern = %q, want %q", spec.Pattern.Pattern, tt.wantPattern)
			}
			if spec.DirOnly != tt.wantDirOnly {
				t.Fatalf("DirOnly = %v, want %v", spec.DirOnly, tt.wantDirOnly)
			}
			if spec.Anchored != tt.wantAnchored {
				t.Fatalf("Anchored = %v, want %v", spec.Anchored, tt.wantAnchored)
			}
			if spec.Pattern.IsLiteral != tt.wantLiteral {
				t.Fatalf("IsLiteral = %v, want %v", spec.Pattern.IsLiteral, tt.wantLiteral)
			}
			if spec.Pattern.HasSlash != tt.wantHasSlash {
				t.Fatalf("HasSlash = %v, want %v", spec.Pattern.HasSlash, tt.wantHasSlash)
			}
			if spec.Pattern.HasDoubleStar != tt.wantHasDoubleStar {
				t.Fatalf("HasDoubleStar = %v, want %v", spec.Pattern.HasDoubleStar, tt.wantHasDoubleStar)
			}
			if spec.Pattern.OnlyStarWildcards != tt.wantOnlyStar {
				t.Fatalf("OnlyStarWildcards = %v, want %v", spec.Pattern.OnlyStarWildcards, tt.wantOnlyStar)
			}
			if spec.Pattern.LiteralPrefix != tt.wantPrefix {
				t.Fatalf("LiteralPrefix = %q, want %q", spec.Pattern.LiteralPrefix, tt.wantPrefix)
			}
			if spec.Pattern.LiteralSuffix != tt.wantSuffix {
				t.Fatalf("LiteralSuffix = %q, want %q", spec.Pattern.LiteralSuffix, tt.wantSuffix)
			}
			if spec.Pattern.PatternValid != tt.wantValid {
				t.Fatalf("PatternValid = %v, want %v", spec.Pattern.PatternValid, tt.wantValid)
			}
		})
	}
}

func TestCompiledPatternMatchCandidate(t *testing.T) {
	cp := compilePattern("foo*bar")
	tests := []struct {
		candidate string
		want      bool
	}{
		{candidate: "foobar", want: true},
		{candidate: "foo-hello-bar", want: true},
		{candidate: "foo", want: false},
		{candidate: "bar", want: false},
		{candidate: "quxbar", want: false},
	}
	for _, tt := range tests {
		if got := cp.matchCandidate(tt.candidate); got != tt.want {
			t.Fatalf("matchCandidate(%q) = %v, want %v", tt.candidate, got, tt.want)
		}
	}

	invalid := compilePattern("[")
	if got := invalid.matchCandidate("anything"); got {
		t.Fatal("invalid glob unexpectedly matched candidate")
	}
}

func TestIsKeptAndIsMatchParity(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
	}{
		{pattern: "*.go", path: "main.go"},
		{pattern: "*.go", path: "src/main.go"},
		{pattern: "/foo.txt", path: "foo.txt"},
		{pattern: "/foo.txt", path: "sub/foo.txt"},
		{pattern: "src/*.go", path: "src/main.go"},
		{pattern: "src/*.go", path: "pkg/main.go"},
		{pattern: "**/test", path: "a/b/test"},
		{pattern: "**/test", path: "test"},
		{pattern: "node_modules/", path: "node_modules/pkg/foo.go"},
		{pattern: "node_modules/", path: "other/foo.go"},
		{pattern: `src\*.go`, path: `src\main.go`},
		{pattern: "[", path: "main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"|"+tt.path, func(t *testing.T) {
			match := IsMatch(tt.pattern, tt.path)
			if got := IsKept(tt.path, []string{tt.pattern}, nil); got != match {
				t.Fatalf("IsKept(include=%q) = %v, want %v (IsMatch parity)", tt.pattern, got, match)
			}
			if got := IsKept(tt.path, nil, []string{tt.pattern}); got != !match {
				t.Fatalf("IsKept(exclude=%q) = %v, want %v (!IsMatch parity)", tt.pattern, got, !match)
			}
		})
	}
}

func TestParseRulesUsesCompiledSpec(t *testing.T) {
	rules := parseRules([]string{
		"foo",
		"*.go",
		"build/",
		"/root.txt",
		"!negated",
	}, "nested", ".gitignore")

	if len(rules) != 5 {
		t.Fatalf("expected 5 rules, got %d", len(rules))
	}
	for i, rule := range rules {
		want := compileSpec(rule.Pattern)
		if !reflect.DeepEqual(rule.Spec, want) {
			t.Fatalf("rule[%d] spec mismatch: got %#v want %#v", i, rule.Spec, want)
		}
		if rule.BasePathPrefix != "nested/" {
			t.Fatalf("rule[%d] BasePathPrefix = %q, want %q", i, rule.BasePathPrefix, "nested/")
		}
	}
}
