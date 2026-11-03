package walker

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := compileOverrides([]Override{{Glob: tt.raw}}).rules[0].Spec
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

func TestReachBelow(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		dir      string
		want     bool
	}{
		{
			name:     "literal file keeps ancestor",
			patterns: []string{"scripts/bpf_doc.py"},
			dir:      "scripts",
			want:     true,
		},
		{
			name:     "literal file prunes unrelated dir",
			patterns: []string{"scripts/bpf_doc.py"},
			dir:      "kernel",
			want:     false,
		},
		{
			name:     "file segment prunes deeper directories",
			patterns: []string{"scripts/*bpf_doc.py"},
			dir:      "scripts/tools",
			want:     false,
		},
		{
			name:     "double-star segment keeps every directory below it",
			patterns: []string{"scripts/**/x.py"},
			dir:      "scripts/a/b",
			want:     true,
		},
		{
			name:     "brace alternative with a slash is kept",
			patterns: []string{"{a/b,c}/x.go"},
			dir:      "a",
			want:     true,
		},
		{
			name:     "floating basename pattern cannot be pruned",
			patterns: []string{"*.go"},
			dir:      "anywhere/deep",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var globs []Override
			for _, p := range tt.patterns {
				globs = append(globs, IncludeOverride(p))
			}
			if got := compileOverrides(globs).reachBelow(tt.dir); got != tt.want {
				t.Fatalf("reachBelow(%v, %q) = %v, want %v", tt.patterns, tt.dir, got, tt.want)
			}
		})
	}
}
