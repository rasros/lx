package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/pkg/lx"
)

func TestStatsFuncs_TokenLabel(t *testing.T) {
	tokenLabel := statsFuncs(lx.TemplateFuncs("")["commafy"].(func(interface{}) string))["tokenLabel"].(func(int64, bool) string)

	if got := tokenLabel(15, false); got != "~15 tokens" {
		t.Errorf("tokenLabel(15,false) = %q, want %q", got, "~15 tokens")
	}
	if got := tokenLabel(1234, false); got != "~1,234 tokens" {
		t.Errorf("tokenLabel(1234,false) = %q", got)
	}
	if strings.ContainsAny(tokenLabel(15, false), "\x1b") {
		t.Errorf("tokenLabel without color must not contain escapes")
	}
	cases := []struct {
		n     int64
		color string
	}{
		{90_000, string(ansi.Green)},
		{150_000, string(ansi.Yellow)},
		{300_000, string(ansi.Red)},
	}
	for _, c := range cases {
		out := tokenLabel(c.n, true)
		if !strings.HasPrefix(out, c.color+string(ansi.Bold)) {
			t.Errorf("tokenLabel(%d,true) = %q, want prefix %q", c.n, out, c.color+string(ansi.Bold))
		}
		if !strings.HasSuffix(out, string(ansi.Reset)) {
			t.Errorf("tokenLabel(%d,true) missing reset", c.n)
		}
	}
}

func renderStats(t *testing.T, r lx.Report) string {
	t.Helper()
	stats, err := compileStats("", "")
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	if err := stats.Execute(&sb, StatsContext{Global: r}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	return sb.String()
}

func TestStatsOmitsCounterLineWhenNothingLeftOut(t *testing.T) {
	got := renderStats(t, lx.Report{Plan: lx.Plan{TotalFiles: 3, TotalSections: 1, TotalSize: 900}, TotalRows: 40})
	for _, unwanted := range []string{"skipped", "binary", "failed"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stats mentions %q with all counters zero:\n%s", unwanted, got)
		}
	}
}

func TestStatsShowsOnlyNonZeroCounters(t *testing.T) {
	got := renderStats(t, lx.Report{Plan: lx.Plan{TotalFiles: 3, SkippedBySize: 2}})
	if !strings.Contains(got, "2 skipped (size)") {
		t.Errorf("stats missing skipped count:\n%s", got)
	}
	if strings.Contains(got, "binary") || strings.Contains(got, "failed") {
		t.Errorf("stats shows zero-valued counters:\n%s", got)
	}
}

func TestStatsSeparatesMultipleCounters(t *testing.T) {
	got := renderStats(t, lx.Report{
		Plan: lx.Plan{TotalFiles: 9, SkippedBySize: 2}, BinaryFiles: 4, FailedFiles: 1,
	})
	want := "2 skipped (size) · 4 binary · 1 failed"
	if !strings.Contains(got, want) {
		t.Errorf("stats counter line = %q, want it to contain %q", got, want)
	}
}

func TestStatsCounterLineHasNoLeadingSeparator(t *testing.T) {
	got := renderStats(t, lx.Report{Plan: lx.Plan{TotalFiles: 5}, BinaryFiles: 3})
	if !strings.Contains(got, "  3 binary\n") {
		t.Errorf("stats counter line malformed:\n%q", got)
	}
}

func TestDefaultStatsTemplate_Render(t *testing.T) {
	stats, err := compileStats("", "")
	if err != nil {
		t.Fatal(err)
	}

	render := func(ctx StatsContext) string {
		var buf bytes.Buffer
		if err := stats.Execute(&buf, ctx); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		return buf.String()
	}

	t.Run("two lines, no color, plural section", func(t *testing.T) {
		out := render(StatsContext{
			Global: lx.Report{
				Plan:          lx.Plan{TotalFiles: 12, TotalSections: 3, TotalSize: 58400},
				TotalRows:     1847,
				TokenEstimate: 14612,
			},
		})
		if strings.ContainsAny(out, "\x1b") {
			t.Errorf("uncolored render must not contain ANSI escapes: %q", out)
		}
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("want 2 lines, got %d: %q", len(lines), out)
		}
		want1 := "▎ 12 files · 1,847 rows · 3 sections · 58.4 kB"
		want2 := "  ~14,612 tokens"
		if lines[0] != want1 {
			t.Errorf("line 1 = %q, want %q", lines[0], want1)
		}
		if lines[1] != want2 {
			t.Errorf("line 2 = %q, want %q", lines[1], want2)
		}
	})

	t.Run("singular pluralization at 1", func(t *testing.T) {
		out := render(StatsContext{
			Global: lx.Report{
				Plan:      lx.Plan{TotalFiles: 1, TotalSections: 1, TotalSize: 27},
				TotalRows: 1, TokenEstimate: 15,
			},
		})
		if !strings.Contains(out, "1 file ·") || !strings.Contains(out, "1 row ·") || !strings.Contains(out, "1 section ·") {
			t.Errorf("singular forms missing: %q", out)
		}
	})

	t.Run("size hidden when unavailable", func(t *testing.T) {
		out := render(StatsContext{
			Global: lx.Report{
				Plan:      lx.Plan{TotalFiles: 1, TotalSections: 1, TotalSize: 0, SizeUnknown: true},
				TotalRows: 50, TokenEstimate: 10982,
			},
		})
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		want1 := "▎ 1 file · 50 rows · 1 section"
		if lines[0] != want1 {
			t.Errorf("line 1 = %q, want %q", lines[0], want1)
		}
		if strings.Contains(out, "-1") {
			t.Errorf("rendered stats must never show a negative size: %q", out)
		}
	})

	t.Run("color path bolds and colors token count", func(t *testing.T) {
		out := render(StatsContext{
			ColorEnabled: true,
			Global: lx.Report{
				Plan:      lx.Plan{TotalFiles: 2, TotalSections: 1, TotalSize: 100},
				TotalRows: 4, TokenEstimate: 250_000,
			},
		})
		if !strings.Contains(out, string(ansi.Red)+string(ansi.Bold)+"~250,000 tokens"+string(ansi.Reset)) {
			t.Errorf("expected red+bold token label in colored render: %q", out)
		}
		if !strings.Contains(out, string(ansi.Cyan)+"▎"+string(ansi.Reset)) {
			t.Errorf("expected accent on leader: %q", out)
		}
	})
}
