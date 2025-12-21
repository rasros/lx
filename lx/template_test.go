package lx

import (
	"bytes"
	"testing"
	"time"
)

func TestIsBinaryData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"simple text", []byte("hello world"), false},
		{"null byte at start", []byte{0x00, 0x61}, true},
		{"null byte later", append(bytes.Repeat([]byte{'a'}, 600), 0x00), true},
		{"invalid utf8", []byte{0xff, 0xfe, 0xfd}, true},
	}

	for _, tt := range tests {
		got := IsBinaryData(tt.data)
		if got != tt.want {
			t.Errorf("IsBinaryData(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

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
