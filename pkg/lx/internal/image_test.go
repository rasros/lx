package internal

import "testing"

func TestIsImage(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"image.png", true},
		{"photo.JPG", true},
		{"vector.svg", false},
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

func TestMIMEType(t *testing.T) {
	cases := map[string]string{
		"a/logo.PNG":  "image/png",
		"photo.jpeg":  "image/jpeg",
		"icon.svg":    "image/svg+xml",
		"archive.bin": "application/octet-stream",
	}
	for path, want := range cases {
		if got := MIMEType(path); got != want {
			t.Errorf("MIMEType(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDataURIEncodesGivenBytes(t *testing.T) {
	got := DataURI("logo.png", []byte("hi"))
	want := "data:image/png;base64,aGk="
	if got != want {
		t.Errorf("DataURI = %q, want %q", got, want)
	}
}
