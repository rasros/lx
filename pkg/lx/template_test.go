package lx

import (
	"testing"
	"time"
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
