package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEstimateLineCount(t *testing.T) {
	smallData := []byte("1\n2\n3\n4\n5\n")
	noNewline := []byte("oneline")

	var largeBuilder bytes.Buffer
	for i := 0; i < 1000; i++ {
		largeBuilder.WriteString("this is a reasonably long line to fill up the buffer fast\n")
	}
	largeData := largeBuilder.Bytes()

	tests := []struct {
		name      string
		data      []byte
		wantCount int
		wantExact bool
	}{
		{"empty", []byte{}, 0, true},
		{"small exact", smallData, 5, true},
		{"no newline", noNewline, 1, false},
		{"large estimate", largeData, 0, false},
	}

	buf := make([]byte, 32*1024)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			count, exact, err := EstimateLineCount(r, int64(len(tt.data)), buf)
			if err != nil {
				t.Fatalf("EstimateLineCount error: %v", err)
			}

			if tt.name != "no newline" && exact != tt.wantExact {
				t.Errorf("exact = %v, want %v", exact, tt.wantExact)
			}

			if tt.wantExact && count != tt.wantCount {
				t.Errorf("count = %d, want %d", count, tt.wantCount)
			}

			if !tt.wantExact && count <= 0 && len(tt.data) > 0 {
				t.Errorf("Estimated count should be > 0, got %d", count)
			}
		})
	}
}

func TestReadHead(t *testing.T) {
	input := "line1\nline2\nline3\n"
	r := strings.NewReader(input)

	got, lines, err := ReadHead(r, 2)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 2 {
		t.Errorf("Expected 2 lines, got %d", lines)
	}
	if string(got) != "line1\nline2\n" {
		t.Errorf("Content mismatch: %q", string(got))
	}

	// Test read past EOF
	r.Reset(input)
	got, lines, err = ReadHead(r, 10)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 3 {
		t.Errorf("Expected 3 lines (EOF), got %d", lines)
	}
}

func TestReadTailSeek(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	content := "1\n2\n3\n4\n5\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(fpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tests := []struct {
		name      string
		n         int
		wantBytes string
	}{
		{"Subset", 2, "4\n5\n"},
		{"Exact size", 5, "1\n2\n3\n4\n5\n"},
		{"More than exist", 10, "1\n2\n3\n4\n5\n"},
		{"Zero", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadTailSeek(f, tt.n)
			if err != nil {
				t.Fatalf("ReadTailSeek error: %v", err)
			}
			if string(got) != tt.wantBytes {
				t.Errorf("Got: %q\nWant: %q", string(got), tt.wantBytes)
			}
		})
	}
}

func TestLineNumberFormatter(t *testing.T) {
	lnf := LineNumberFormatter{
		Head:      []byte("one\n"),
		Gap:       []byte("...\n"),
		Tail:      []byte("ten\n"),
		TotalRows: 10,
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%v", lnf)
	got := buf.String()

	if !strings.Contains(got, " 1: one") {
		t.Error("Missing single digit padded line number")
	}
	if !strings.Contains(got, "10: ten") {
		t.Error("Missing double digit line number")
	}
}
