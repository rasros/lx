package lx

import "testing"

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
