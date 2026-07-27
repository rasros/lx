package templatex

import (
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/internal"
)

func compileForFormat(t *testing.T, format string) *core.TemplateEngine {
	t.Helper()
	cfg := core.NewConfig()
	cfg.OutputFormat = format
	engine, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile(%s) failed: %v", format, err)
	}
	return engine
}

func TestCompileProvidesMetaForEveryFormat(t *testing.T) {
	for _, format := range []string{"markdown", "xml", "html", "bare"} {
		if engine := compileForFormat(t, format); engine.Meta == nil {
			t.Errorf("format %s: Meta template is nil", format)
		}
	}
}

func TestMetaTemplateRendersBody(t *testing.T) {
	cases := map[string]string{
		"markdown": "OS: linux",
		"xml":      "<system_context>",
		"html":     "<pre class=\"system-context\">",
	}

	for format, want := range cases {
		engine := compileForFormat(t, format)
		var sb strings.Builder
		err := engine.Meta.Execute(&sb, core.MetaContext{
			Body:   "OS: linux",
			Fields: map[string]string{"os": "linux"},
		})
		if err != nil {
			t.Errorf("format %s: Execute failed: %v", format, err)
			continue
		}
		if !strings.Contains(sb.String(), want) {
			t.Errorf("format %s: output %q missing %q", format, sb.String(), want)
		}
	}
}

func TestMetaTemplateExposesFields(t *testing.T) {
	cfg := core.NewConfig()
	cfg.MetaTemplate = "{{ .Fields.os }}/{{ .Fields.arch }}"
	engine, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	var sb strings.Builder
	err = engine.Meta.Execute(&sb, core.MetaContext{
		Fields: map[string]string{"os": "linux", "arch": "amd64"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if got := sb.String(); got != "linux/amd64" {
		t.Errorf("got %q, want %q", got, "linux/amd64")
	}
}

func renderStats(t *testing.T, g core.GlobalContext) string {
	t.Helper()
	engine := compileForFormat(t, "markdown")
	var sb strings.Builder
	if err := engine.Stats.Execute(&sb, core.StatsContext{Global: g}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	return sb.String()
}

func TestStatsOmitsCounterLineWhenNothingLeftOut(t *testing.T) {
	got := renderStats(t, core.GlobalContext{TotalFiles: 3, TotalRows: 40, TotalSections: 1, TotalSize: 900})
	for _, unwanted := range []string{"skipped", "binary", "failed"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stats mentions %q with all counters zero:\n%s", unwanted, got)
		}
	}
}

func TestStatsShowsOnlyNonZeroCounters(t *testing.T) {
	got := renderStats(t, core.GlobalContext{TotalFiles: 3, SkippedBySize: 2})
	if !strings.Contains(got, "2 skipped (size)") {
		t.Errorf("stats missing skipped count:\n%s", got)
	}
	if strings.Contains(got, "binary") || strings.Contains(got, "failed") {
		t.Errorf("stats shows zero-valued counters:\n%s", got)
	}
}

func TestStatsSeparatesMultipleCounters(t *testing.T) {
	got := renderStats(t, core.GlobalContext{
		TotalFiles: 9, SkippedBySize: 2, BinaryFiles: 4, FailedFiles: 1,
	})
	want := "2 skipped (size) · 4 binary · 1 failed"
	if !strings.Contains(got, want) {
		t.Errorf("stats counter line = %q, want it to contain %q", got, want)
	}
}

func TestStatsCounterLineHasNoLeadingSeparator(t *testing.T) {
	got := renderStats(t, core.GlobalContext{TotalFiles: 5, BinaryFiles: 3})
	if !strings.Contains(got, "  3 binary\n") {
		t.Errorf("stats counter line malformed:\n%q", got)
	}
}

// Content is either a string or a LineNumberFormatter; helpers must treat both
// alike. Reaching for %v on the formatter silently skipped escaping.
func TestEscapeHandlesLineNumberedContent(t *testing.T) {
	escape := templateFuncs()["escape"].(func(interface{}) string)

	plain := escape("<script>alert(1)</script>\n")
	numbered := escape(internal.LineNumberFormatter{
		Head:      []byte("<script>alert(1)</script>\n"),
		TotalRows: 1,
	})

	if strings.Contains(plain, "<script>") {
		t.Errorf("string content not escaped: %q", plain)
	}
	if strings.Contains(numbered, "<script>") {
		t.Errorf("line-numbered content not escaped: %q", numbered)
	}
	if !strings.Contains(numbered, "1: &lt;script&gt;") {
		t.Errorf("line-numbered content lost its numbering or escaping: %q", numbered)
	}
}

func TestEndNewlineHandlesLineNumberedContent(t *testing.T) {
	endNewline := templateFuncs()["endNewline"].(func(interface{}) string)

	got := endNewline(internal.LineNumberFormatter{Head: []byte("alpha"), TotalRows: 1})
	if got != "1: alpha\n" {
		t.Errorf("endNewline = %q, want %q", got, "1: alpha\n")
	}
}
