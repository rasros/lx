package walker

import (
	"strings"
	"testing"
)

func TestMatch_BasePathScopedRule(t *testing.T) {
	rules := parseRules([]string{"foo.txt"}, "sub", ".gitignore")
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	rule := rules[0]

	if !newPathMatcher("sub/foo.txt").match(rule, false) {
		t.Fatal("expected scoped rule to match path within base path")
	}
	if newPathMatcher("other/foo.txt").match(rule, false) {
		t.Fatal("expected scoped rule not to match path outside base path")
	}
}

func TestMatch_DirOnlyRequiresDirEntry(t *testing.T) {
	rules := parseRules([]string{"build/"}, "", ".gitignore")
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	rule := rules[0]

	if !newPathMatcher("build").match(rule, true) {
		t.Fatal("expected dir-only rule to match directory")
	}
	if newPathMatcher("build").match(rule, false) {
		t.Fatal("expected dir-only rule not to match non-directory entry")
	}
}

func TestLastMatch_NegationOverridesPreviousRule(t *testing.T) {
	rules := parseRules([]string{"*.log", "!keep.log"}, "", ".gitignore")

	m, reason := lastMatch("app.log", false, rules, true)
	if m != matchIgnore {
		t.Fatal("expected app.log to be ignored")
	}
	if !strings.Contains(reason, "*.log") {
		t.Fatalf("unexpected ignore reason: %q", reason)
	}

	m, reason = lastMatch("keep.log", false, rules, true)
	if m != matchKeep {
		t.Fatal("expected keep.log to be un-ignored by negation rule")
	}
	if reason != "" {
		t.Fatalf("expected empty reason after negation, got %q", reason)
	}
}

func TestLastMatch_OmitsReasonWhenNotRequested(t *testing.T) {
	rules := parseRules([]string{"*.log"}, "", ".gitignore")
	m, reason := lastMatch("app.log", false, rules, false)
	if m != matchIgnore {
		t.Fatal("expected app.log to be ignored")
	}
	if reason != "" {
		t.Fatalf("expected empty reason when wantReason=false, got %q", reason)
	}
}

func TestParseRules_Escapes(t *testing.T) {
	tests := []struct {
		line, pattern string
		negate        bool
	}{
		{`trail\ `, "trail ", false},
		{"trail2   ", "trail2", false},
		{"crlf\r", "crlf", false},
		{`\#hash`, "#hash", false},
		{`\!bang`, "!bang", false},
		{"!keep", "keep", true},
	}
	for _, tt := range tests {
		rules := parseRules([]string{tt.line}, "", "test")
		if len(rules) != 1 || rules[0].Pattern != tt.pattern || rules[0].Negate != tt.negate {
			t.Errorf("parseRules(%q) = %+v, want pattern %q negate %v", tt.line, rules, tt.pattern, tt.negate)
		}
	}
}

func TestLastMatch_BackslashEscapes(t *testing.T) {
	rules := parseRules([]string{`\*.go`, `a\ b.go`}, "", ".gitignore")
	for path, want := range map[string]bool{"*.go": true, "x.go": false, "a b.go": true, `a\ b.go`: false} {
		if ignored := isIgnored(path, false, rules); ignored != want {
			t.Errorf("%q ignored = %v, want %v", path, ignored, want)
		}
	}
}

func TestLastMatch_FloatingPatternMatchesBaseNameOnly(t *testing.T) {
	rules := parseRules([]string{"tags", "!tags/"}, "", ".gitignore")
	if isIgnored("tags", true, rules) {
		t.Fatal("expected the negated directory to be re-included")
	}
	if isIgnored("tags/Makefile", false, rules) {
		t.Fatal("expected a file in a re-included directory not to match the directory's name")
	}
}

func TestLastMatch_NegationKeeps(t *testing.T) {
	rules := parseRules([]string{".*", "!.keep"}, "", ".gitignore")
	if m, _ := lastMatch(".keep", false, rules, false); m != matchKeep {
		t.Fatal("expected .keep to be kept by the negation")
	}
	if m, _ := lastMatch(".other", false, rules, false); m != matchIgnore {
		t.Fatal("expected .other to be decided by the ignore rule")
	}
}

func isIgnored(path string, isDir bool, rules []Rule) bool {
	m, _ := lastMatch(path, isDir, rules, false)
	return m == matchIgnore
}
