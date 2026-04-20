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

	if !match(rule, "sub/foo.txt", false) {
		t.Fatal("expected scoped rule to match path within base path")
	}
	if match(rule, "other/foo.txt", false) {
		t.Fatal("expected scoped rule not to match path outside base path")
	}
}

func TestMatch_DirOnlyRequiresDirEntry(t *testing.T) {
	rules := parseRules([]string{"build/"}, "", ".gitignore")
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	rule := rules[0]

	if !match(rule, "build", true) {
		t.Fatal("expected dir-only rule to match directory")
	}
	if match(rule, "build", false) {
		t.Fatal("expected dir-only rule not to match non-directory entry")
	}
}

func TestCheckIgnore_NegationOverridesPreviousRule(t *testing.T) {
	rules := parseRules([]string{"*.log", "!keep.log"}, "", ".gitignore")

	ignored, reason := checkIgnore("app.log", false, rules, false, true)
	if !ignored {
		t.Fatal("expected app.log to be ignored")
	}
	if !strings.Contains(reason, "*.log") {
		t.Fatalf("unexpected ignore reason: %q", reason)
	}

	ignored, reason = checkIgnore("keep.log", false, rules, false, true)
	if ignored {
		t.Fatal("expected keep.log to be un-ignored by negation rule")
	}
	if reason != "" {
		t.Fatalf("expected empty reason after negation, got %q", reason)
	}
}

func TestCheckIgnore_ParentIgnoredWithNegation(t *testing.T) {
	rules := parseRules([]string{"!keep.go"}, "", ".gitignore")

	ignored, _ := checkIgnore("other.go", false, rules, true, false)
	if !ignored {
		t.Fatal("expected parent ignored state to persist when negation does not match")
	}

	ignored, _ = checkIgnore("keep.go", false, rules, true, false)
	if ignored {
		t.Fatal("expected matching negation to clear parent ignored state")
	}
}

func TestCheckIgnore_OmitsReasonWhenNotRequested(t *testing.T) {
	rules := parseRules([]string{"*.log"}, "", ".gitignore")
	ignored, reason := checkIgnore("app.log", false, rules, false, false)
	if !ignored {
		t.Fatal("expected app.log to be ignored")
	}
	if reason != "" {
		t.Fatalf("expected empty reason when wantReason=false, got %q", reason)
	}
}

func TestHasNestedException(t *testing.T) {
	rules := parseRules([]string{"!build/output.txt"}, "", ".gitignore")
	if !hasNestedException("build", rules) {
		t.Fatal("expected nested exception for parent directory")
	}

	rules = parseRules([]string{"!docs/*.md"}, "", ".gitignore")
	if hasNestedException("build", rules) {
		t.Fatal("did not expect nested exception for unrelated directory")
	}
}
