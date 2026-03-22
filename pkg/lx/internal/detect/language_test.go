package detect

import (
	"bytes"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path, data, want string
	}{
		{"main.go", "", "go"},
		{"script.py", "", "python"},
		{"README.md", "", "markdown"},
		{"styles.css", "", "css"},
		{"index.html", "", "html"},
		{"component.tsx", "", "tsx"},
		{"data.json", "", "json"},
		{"infra.tf", "", "hcl"},
		{"test.h", "", "c"},
		{"Dockerfile", "", "dockerfile"},
		{"Makefile", "", "makefile"},
		{"go.mod", "", "gomod"},
		{".gitignore", "", "gitignore"},
		{".bashrc", "", "bash"},
		{"script", "#!/bin/bash\n", "bash"},
		{"tool", "#!/usr/bin/env python\n", "python"},
		{"idx", "#!/usr/bin/env node\n", "javascript"},
		{"none", "", ""},
	}

	for _, tt := range tests {
		if got := DetectLanguage(tt.path, []byte(tt.data)); got != tt.want {
			t.Errorf("DetectLanguage(%q, %q) = %q, want %q", tt.path, tt.data, got, tt.want)
		}
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"simple text", []byte("hello world"), false},
		{"null byte at start", []byte{0x00, 0x61}, true},
		{"null byte later", append([]byte{'a'}, 0x00), true},
		{"invalid utf8", []byte{0xff, 0xfe, 0xfd}, true},
	}

	for _, tt := range tests {
		got := IsBinary(tt.data)
		if got != tt.want {
			t.Errorf("IsBinary(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsBinary_Truncation(t *testing.T) {
	// 1021 bytes of pure ASCII
	base := bytes.Repeat([]byte("a"), 1021)

	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "Exact fit ASCII",
			data: append(base, []byte("aaa")...),
			want: false,
		},
		{
			name: "Truncated 3-byte char missing 1 byte",
			data: append(bytes.Repeat([]byte("a"), 1022), []byte("✓")[:2]...),
			want: false,
		},
		{
			name: "Truncated 4-byte char missing 2 bytes",
			data: append(bytes.Repeat([]byte("a"), 1022), []byte("🌍")[:2]...),
			want: false,
		},
		{
			name: "Truncated 4-byte char missing 1 byte",
			data: append(bytes.Repeat([]byte("a"), 1021), []byte("🌍")[:3]...),
			want: false,
		},
		{
			name: "Genuine binary contains null",
			data: append(bytes.Repeat([]byte("a"), 1023), 0x00),
			want: true,
		},
		{
			name: "Genuine invalid UTF-8 in middle",
			data: append(bytes.Repeat([]byte("a"), 500), append([]byte{0xff, 0xfe}, bytes.Repeat([]byte("a"), 522)...)...),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBinary(tt.data); got != tt.want {
				t.Errorf("IsBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}
