package templatex

import (
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
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

func TestCompileProvidesMetaAndReportForEveryFormat(t *testing.T) {
	for _, format := range []string{"markdown", "xml", "html", "bare"} {
		engine := compileForFormat(t, format)
		if engine.Meta == nil {
			t.Errorf("format %s: Meta template is nil", format)
		}
		if engine.Report == nil {
			t.Errorf("format %s: Report template is nil", format)
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

func TestReportTemplateRendersRowsAndBreakdown(t *testing.T) {
	engine := compileForFormat(t, "markdown")

	ctx := core.ReportContext{
		Files: []core.FileReport{
			{Path: "docs/manual.pdf", OriginalSize: 12_400_000, RenderedSize: 180_000, Tokens: 45_000},
			{Path: "src/main.go", OriginalSize: 45_000, RenderedSize: 45_000, Tokens: 11_200},
		},
		Top: []core.FileReport{{Path: "docs/manual.pdf"}, {Path: "src/main.go"}},
		Global: core.GlobalContext{
			TotalFiles:    2,
			SkippedBySize: 3,
			BinaryFiles:   1,
			FailedFiles:   0,
			TokenEstimate: 56_200,
		},
	}

	var sb strings.Builder
	if err := engine.Report.Execute(&sb, ctx); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got := sb.String()
	for _, want := range []string{
		"docs/manual.pdf",
		"12.4 MB",
		"180.0 kB",
		"45,000",
		"src/main.go",
		"Largest: docs/manual.pdf, src/main.go",
		"2 processed",
		"3 skipped (size)",
		"1 binary",
		"0 failed",
		"~56,200 tokens",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report output missing %q:\n%s", want, got)
		}
	}
}

// An empty report must not emit a headerless table.
func TestReportTemplateOmitsTableWhenNoFiles(t *testing.T) {
	engine := compileForFormat(t, "markdown")

	var sb strings.Builder
	if err := engine.Report.Execute(&sb, core.ReportContext{}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got := sb.String()
	if strings.Contains(got, "| File |") {
		t.Errorf("empty report rendered a table header:\n%s", got)
	}
	if !strings.Contains(got, "0 processed") {
		t.Errorf("empty report missing breakdown:\n%s", got)
	}
}

func TestCustomReportTemplateOverridesDefault(t *testing.T) {
	cfg := core.NewConfig()
	cfg.ReportTemplate = "{{ len .Files }} files"
	engine, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	var sb strings.Builder
	err = engine.Report.Execute(&sb, core.ReportContext{
		Files: []core.FileReport{{Path: "a.go"}, {Path: "b.go"}},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if got := sb.String(); got != "2 files" {
		t.Errorf("got %q, want %q", got, "2 files")
	}
}
