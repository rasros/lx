package lx

import (
	"testing"
	"time"
)

func TestTemplateFuncs_Humanize(t *testing.T) {
	humanize := TemplateFuncs()["humanize"].(func(int64) string)

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
		got := humanize(tt.in)
		if got != tt.want {
			t.Errorf("humanize(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTemplateFuncs_Date(t *testing.T) {
	dateFn := TemplateFuncs()["date"].(func(string, time.Time) string)

	ts := time.Date(2025, 12, 17, 14, 0, 0, 0, time.UTC)
	got := dateFn("2006-01-02", ts)
	if got != "2025-12-17" {
		t.Errorf("date() = %q, want 2025-12-17", got)
	}
}
