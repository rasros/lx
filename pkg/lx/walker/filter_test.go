package walker

import (
	"path/filepath"
	"testing"
)

func TestIsKept(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		includes []string
		excludes []string
		want     bool
	}{
		{
			name:     "no filters",
			path:     "main.go",
			includes: nil,
			excludes: nil,
			want:     true,
		},
		{
			name:     "include match exact",
			path:     "main.go",
			includes: []string{"main.go"},
			excludes: nil,
			want:     true,
		},
		{
			name:     "include match glob",
			path:     "main.go",
			includes: []string{"*.go"},
			excludes: nil,
			want:     true,
		},
		{
			name:     "include mismatch",
			path:     "readme.md",
			includes: []string{"*.go"},
			excludes: nil,
			want:     false,
		},
		{
			name:     "exclude match glob",
			path:     "main_test.go",
			includes: nil,
			excludes: []string{"*_test.go"},
			want:     false,
		},
		{
			name:     "exclude mismatch",
			path:     "main.go",
			includes: nil,
			excludes: []string{"*_test.go"},
			want:     true,
		},
		{
			name:     "include match, exclude match (exclude wins)",
			path:     "main_test.go",
			includes: []string{"*.go"},
			excludes: []string{"*_test.go"},
			want:     false,
		},
		{
			name:     "include match, exclude mismatch",
			path:     "main.go",
			includes: []string{"*.go"},
			excludes: []string{"*_test.go"},
			want:     true,
		},
		{
			name:     "path match with separator (include)",
			path:     filepath.Join("cmd", "main.go"),
			includes: []string{"cmd/*"},
			excludes: nil,
			want:     true,
		},
		{
			name:     "path match with separator (exclude)",
			path:     filepath.Join("vendor", "foo.go"),
			includes: nil,
			excludes: []string{"vendor/*"},
			want:     false,
		},
		{
			name:     "mixed filename and path globs",
			path:     filepath.Join("internal", "foo_test.go"),
			includes: []string{"internal/*"},
			excludes: []string{"*_test.go"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsKept(tt.path, tt.includes, tt.excludes)
			if got != tt.want {
				t.Errorf("IsKept(%q, inc=%v, exc=%v) = %v, want %v",
					tt.path, tt.includes, tt.excludes, got, tt.want)
			}
		})
	}
}
