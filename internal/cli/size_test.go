package cli

import (
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

func TestParseSizeLimit(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1", 1},
		{"4096", 4096},
		{"512k", 512_000},
		{"512K", 512_000},
		{"512kb", 512_000},
		{"512KB", 512_000},
		{"2m", 2_000_000},
		{"2M", 2_000_000},
		{"2mb", 2_000_000},
		{"1g", 1_000_000_000},
		{"1G", 1_000_000_000},
		{"1GB", 1_000_000_000},
		{"900b", 900},
		{"900B", 900},
		{"  512k  ", 512_000},
	}

	for _, c := range cases {
		got, err := parseSizeLimit(c.in)
		if err != nil {
			t.Errorf("parseSizeLimit(%q) returned error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseSizeLimit(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseSizeLimitErrors(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"notasize",
		"k",
		"kb",
		"0",
		"0k",
		"-1",
		"-5k",
		"1.5M",
		"512 k",
		"12x",
		"9223372036854775807k",
	}

	for _, in := range cases {
		if got, err := parseSizeLimit(in); err == nil {
			t.Errorf("parseSizeLimit(%q) = %d, want error", in, got)
		}
	}
}

func TestMaxSizeFlagParsing(t *testing.T) {
	defaultRunCfg := lx.RunnerConfig{Head: -1, Tail: 0}
	parsed, err := Parse([]string{"-m", "512k", "."}, definitions)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	sections := parseSections(parsed.Ops, defaultRunCfg)
	if len(sections) != 1 {
		t.Fatalf("len(sections) = %d, want 1", len(sections))
	}
	if got := sections[0].RunCfg.MaxSize; got != 512_000 {
		t.Errorf("MaxSize = %d, want 512000", got)
	}
}

func TestMaxSizeFlagAttachedShortForm(t *testing.T) {
	defaultRunCfg := lx.RunnerConfig{Head: -1, Tail: 0}
	parsed, err := Parse([]string{"-m2M", "."}, definitions)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	sections := parseSections(parsed.Ops, defaultRunCfg)
	if got := sections[0].RunCfg.MaxSize; got != 2_000_000 {
		t.Errorf("MaxSize = %d, want 2000000", got)
	}
}

func TestMaxSizeFlagRejectsInvalidValue(t *testing.T) {
	if _, err := Parse([]string{"-m", "notasize", "."}, definitions); err == nil {
		t.Fatal("Parse accepted an invalid size, want error")
	}
}

// A second -m after a file argument opens a new section, so the limit is
// re-derived from the default RunnerConfig rather than inherited.
func TestMaxSizeIsPerSection(t *testing.T) {
	defaultRunCfg := lx.RunnerConfig{Head: -1, Tail: 0}
	parsed, err := Parse([]string{"-m", "20", ".", "-m", "1k", "large.txt"}, definitions)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	sections := parseSections(parsed.Ops, defaultRunCfg)
	if len(sections) != 2 {
		t.Fatalf("len(sections) = %d, want 2", len(sections))
	}
	if got := sections[0].RunCfg.MaxSize; got != 20 {
		t.Errorf("first section MaxSize = %d, want 20", got)
	}
	if got := sections[1].RunCfg.MaxSize; got != 1000 {
		t.Errorf("second section MaxSize = %d, want 1000", got)
	}
}

func TestNoMaxSizeLeavesLimitUnset(t *testing.T) {
	defaultRunCfg := lx.RunnerConfig{Head: -1, Tail: 0}
	parsed, err := Parse([]string{"."}, definitions)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	sections := parseSections(parsed.Ops, defaultRunCfg)
	if got := sections[0].RunCfg.MaxSize; got != 0 {
		t.Errorf("MaxSize = %d, want 0", got)
	}
}
