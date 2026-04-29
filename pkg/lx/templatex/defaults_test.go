package templatex

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
)

func TestTemplateFuncs_Humanize(t *testing.T) {
	humanize := templateFuncs()["humanize"].(func(int64) string)

	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1000, "1.0 kB"},
		{1500, "1.5 kB"},
		{1000000, "1.0 MB"},
		{1000000000, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := humanize(tt.in)
			if got != tt.want {
				t.Errorf("humanize(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTemplateFuncs_Date(t *testing.T) {
	dateFn := templateFuncs()["date"].(func(string, time.Time) string)

	ts := time.Date(2025, 12, 17, 14, 0, 0, 0, time.UTC)
	got := dateFn("2006-01-02", ts)
	if got != "2025-12-17" {
		t.Errorf("date() = %q, want 2025-12-17", got)
	}
}

func TestTemplateFuncs_EndNewline(t *testing.T) {
	endNewline := templateFuncs()["endNewline"].(func(interface{}) string)

	tests := []struct {
		in   interface{}
		want string
	}{
		{"hello", "hello\n"},
		{"already\n", "already\n"},
		{"", ""},
		{42, "42"}, // non-string falls back to Sprintf
	}

	for _, tt := range tests {
		got := endNewline(tt.in)
		if got != tt.want {
			t.Errorf("endNewline(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTemplateFuncs_Commafy(t *testing.T) {
	commafy := templateFuncs()["commafy"].(func(interface{}) string)
	tests := []struct {
		in   interface{}
		want string
	}{
		{0, "0"},
		{42, "42"},
		{1000, "1,000"},
		{int64(1234567), "1,234,567"},
		{int64(-1234567), "-1,234,567"},
		{12345, "12,345"},
	}
	for _, tt := range tests {
		got := commafy(tt.in)
		if got != tt.want {
			t.Errorf("commafy(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTemplateFuncs_TokenLabel(t *testing.T) {
	tokenLabel := templateFuncs()["tokenLabel"].(func(int64, bool) string)

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
		{90_000, ansiGreen},
		{150_000, ansiYellow},
		{300_000, ansiRed},
	}
	for _, c := range cases {
		out := tokenLabel(c.n, true)
		if !strings.HasPrefix(out, c.color+ansiBold) {
			t.Errorf("tokenLabel(%d,true) = %q, want prefix %q", c.n, out, c.color+ansiBold)
		}
		if !strings.HasSuffix(out, ansiReset) {
			t.Errorf("tokenLabel(%d,true) missing reset", c.n)
		}
	}
}

func TestTemplateFuncs_Plural(t *testing.T) {
	plural := templateFuncs()["plural"].(func(interface{}, string, string) string)
	if got := plural(1, "file", "files"); got != "file" {
		t.Errorf("plural(1) = %q", got)
	}
	if got := plural(int64(2), "row", "rows"); got != "rows" {
		t.Errorf("plural(2) = %q", got)
	}
	if got := plural(0, "section", "sections"); got != "sections" {
		t.Errorf("plural(0) = %q", got)
	}
}

func TestTemplateFuncs_Escape(t *testing.T) {
	escape := templateFuncs()["escape"].(func(interface{}) string)

	tests := []struct {
		in   interface{}
		want string
	}{
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"safe text", "safe text"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&#34;quoted&#34;"},
		{99, "99"}, // non-string falls back to Sprintf
	}

	for _, tt := range tests {
		got := escape(tt.in)
		if got != tt.want {
			t.Errorf("escape(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDefaultStatsTemplate_Render(t *testing.T) {
	cfg := core.NewConfig()
	engine, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	render := func(ctx core.StatsContext) string {
		var buf bytes.Buffer
		if err := engine.Stats.Execute(&buf, ctx); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		return buf.String()
	}

	t.Run("two lines, no color, plural section", func(t *testing.T) {
		out := render(core.StatsContext{
			Global: core.GlobalContext{
				TotalFiles:    12,
				TotalRows:     1847,
				TotalSections: 3,
				TotalSize:     58400,
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
		out := render(core.StatsContext{
			Global: core.GlobalContext{
				TotalFiles: 1, TotalRows: 1, TotalSections: 1, TotalSize: 27, TokenEstimate: 15,
			},
		})
		if !strings.Contains(out, "1 file ·") || !strings.Contains(out, "1 row ·") || !strings.Contains(out, "1 section ·") {
			t.Errorf("singular forms missing: %q", out)
		}
	})

	t.Run("color path bolds and colors token count", func(t *testing.T) {
		out := render(core.StatsContext{
			ColorEnabled: true,
			Global: core.GlobalContext{
				TotalFiles: 2, TotalRows: 4, TotalSections: 1, TotalSize: 100, TokenEstimate: 250_000,
			},
		})
		if !strings.Contains(out, ansiRed+ansiBold+"~250,000 tokens"+ansiReset) {
			t.Errorf("expected red+bold token label in colored render: %q", out)
		}
		if !strings.Contains(out, ansiCyan+"▎"+ansiReset) {
			t.Errorf("expected accent on leader: %q", out)
		}
	})
}
