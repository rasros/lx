package lx

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEstimateLineCount(t *testing.T) {
	// 4096 is the sample size in lines.go
	smallData := []byte("1\n2\n3\n4\n5\n")

	// Create large data > 4096 bytes
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
		{"large estimate", largeData, 0, false}, // 0 indicates we don't check specific count, just !exact
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			count, exact, err := EstimateLineCount(r, int64(len(tt.data)))
			if err != nil {
				t.Fatalf("EstimateLineCount error: %v", err)
			}

			if exact != tt.wantExact {
				t.Errorf("EstimateLineCount exact = %v, want %v", exact, tt.wantExact)
			}

			if tt.wantExact && count != tt.wantCount {
				t.Errorf("EstimateLineCount count = %d, want %d", count, tt.wantCount)
			}

			// For estimation, we just ensure it returns a positive number
			if !tt.wantExact && count <= 0 {
				t.Errorf("EstimateLineCount estimated count <= 0: %d", count)
			}
		})
	}
}

func TestReadHead(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5\n"

	tests := []struct {
		name      string
		n         int
		wantBytes string
		wantLines int
	}{
		{
			name:      "read subset",
			n:         2,
			wantBytes: "line1\nline2\n",
			wantLines: 2,
		},
		{
			name:      "read all exactly",
			n:         5,
			wantBytes: input,
			wantLines: 5,
		},
		{
			name:      "read more than available",
			n:         10,
			wantBytes: input,
			wantLines: 5,
		},
		{
			name:      "read unlimited (-1)",
			n:         -1,
			wantBytes: input,
			wantLines: 5,
		},
		{
			name:      "read zero",
			n:         0,
			wantBytes: "",
			wantLines: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(input)
			got, lines, err := ReadHead(r, tt.n)
			if err != nil {
				t.Fatalf("ReadHead error: %v", err)
			}

			if string(got) != tt.wantBytes {
				t.Errorf("ReadHead content mismatch.\nGot:\n%q\nWant:\n%q", string(got), tt.wantBytes)
			}
			if lines != tt.wantLines {
				t.Errorf("ReadHead lines = %d, want %d", lines, tt.wantLines)
			}
		})
	}
}

func TestReadTailSeek(t *testing.T) {
	// ReadTailSeek requires an *os.File, so we must write to disk
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	content := "1\n2\n3\n4\n5\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		n         int
		wantBytes string
	}{
		{"subset", 2, "4\n5\n"},
		{"all", 5, "1\n2\n3\n4\n5\n"},
		{"more than all", 10, "1\n2\n3\n4\n5\n"},
		{"zero", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(fpath)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			got, err := ReadTailSeek(f, tt.n)
			if err != nil {
				t.Fatalf("ReadTailSeek error: %v", err)
			}

			if string(got) != tt.wantBytes {
				t.Errorf("ReadTailSeek mismatch.\nGot: %q\nWant: %q", string(got), tt.wantBytes)
			}
		})
	}
}

func TestLineNumberFormatter(t *testing.T) {
	// This tests the fmt.Formatter interface implementation
	lnf := LineNumberFormatter{
		Head:      []byte("one\ntwo\n"),
		Gap:       []byte("... gap ...\n"),
		Tail:      []byte("nine\nten\n"),
		TotalRows: 10,
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%v", lnf)
	got := buf.String()

	// We expect padding for width 2 (since TotalRows is 10)
	// 1: one
	// 2: two
	// ... gap ...
	// 9: nine
	//10: ten

	expects := []string{
		" 1: one",
		" 2: two",
		"... gap ...",
		" 9: nine",
		"10: ten",
	}

	for _, exp := range expects {
		if !strings.Contains(got, exp) {
			t.Errorf("Formatter output missing %q. Got:\n%s", exp, got)
		}
	}
}
