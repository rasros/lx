package lx

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
		{"dockerfile", "", "dockerfile"},
		{"Makefile", "", "makefile"},
		{"go.mod", "", "gomod"},
		{".gitignore", "", "gitignore"},
		{".bashrc", "", "bash"},
		{".env", "", "bash"},
		{"Dockerfile.dev", "", "dockerfile"},
		{"Dockerfile.prod", "", "dockerfile"},
		{"dev.Dockerfile", "", "dockerfile"},
		{"script", "#!/bin/bash\n", "bash"},
		{"tool", "#!/usr/bin/env python\n", "python"},
		{"idx", "#!/usr/bin/env node\n", "javascript"},
		{"none", "", ""},
		{"script.go", "#!/bin/bash", "go"},
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
		{"null byte later", append(bytes.Repeat([]byte{'a'}, 600), 0x00), true},
		{"invalid utf8", []byte{0xff, 0xfe, 0xfd}, true},
	}

	for _, tt := range tests {
		got := IsBinary(tt.data)
		if got != tt.want {
			t.Errorf("IsBinary(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
