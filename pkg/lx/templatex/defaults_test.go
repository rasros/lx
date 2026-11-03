package templatex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
)

func TestTemplateFuncs_Humanize(t *testing.T) {
	humanize := templateFuncs(&core.TemplateEngine{})["humanize"].(func(int64) string)

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
	dateFn := templateFuncs(&core.TemplateEngine{})["date"].(func(string, time.Time) string)

	ts := time.Date(2025, 12, 17, 14, 0, 0, 0, time.UTC)
	got := dateFn("2006-01-02", ts)
	if got != "2025-12-17" {
		t.Errorf("date() = %q, want 2025-12-17", got)
	}
}

func TestTemplateFuncs_EndNewline(t *testing.T) {
	endNewline := templateFuncs(&core.TemplateEngine{})["endNewline"].(func(interface{}) string)

	tests := []struct {
		in   interface{}
		want string
	}{
		{"hello", "hello\n"},
		{"already\n", "already\n"},
		{"", ""},
		{42, "42\n"}, // every value gets the newline, not just strings
	}

	for _, tt := range tests {
		got := endNewline(tt.in)
		if got != tt.want {
			t.Errorf("endNewline(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTemplateFuncs_Commafy(t *testing.T) {
	commafy := templateFuncs(&core.TemplateEngine{})["commafy"].(func(interface{}) string)
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

func TestTemplateFuncs_Plural(t *testing.T) {
	plural := templateFuncs(&core.TemplateEngine{})["plural"].(func(interface{}, string, string) string)
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
	escape := templateFuncs(&core.TemplateEngine{})["escape"].(func(interface{}) string)

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

// dataURI reads a relative path from the engine's Dir, not the process's
// working directory.
func TestDataURIResolvesAgainstDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "logo.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	dataURI := templateFuncs(&core.TemplateEngine{Dir: dir})["dataURI"].(func(string) (string, error))
	if got, _ := dataURI("logo.svg"); !strings.HasPrefix(got, "data:") {
		t.Errorf("dataURI(logo.svg) = %q, want a data URI of the file in Dir", got)
	}
	if got, _ := dataURI("missing.svg"); got != "" {
		t.Errorf("dataURI(missing.svg) = %q, want empty", got)
	}
}
