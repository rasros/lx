package detect

import "testing"

func TestIsImage(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"image.png", true},
		{"photo.JPG", true},
		{"vector.svg", true},
		{"icon.ico", true},
		{"archive.zip", false},
		{"source.go", false},
		{"image.png.bak", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := IsImage(tt.filename); got != tt.want {
				t.Errorf("IsImage(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}
