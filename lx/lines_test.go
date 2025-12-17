package lx

import (
	"bytes"
	"testing"
)

func TestCountLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "empty",
			input: "",
			want:  0,
		},
		{
			name:  "single line without newline",
			input: "one line",
			want:  1,
		},
		{
			name:  "single line with newline",
			input: "one line\n",
			want:  1,
		},
		{
			name:  "multiple lines with final newline",
			input: "a\nb\nc\n",
			want:  3,
		},
		{
			name:  "multiple lines without final newline",
			input: "a\nb\nc",
			want:  3,
		},
	}

	for _, tt := range tests {
		got := countLines([]byte(tt.input))
		if got != tt.want {
			t.Errorf("countLines(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "single line without newline",
			input: "one line",
			want:  []string{"one line"},
		},
		{
			name:  "single line with newline",
			input: "one line\n",
			want:  []string{"one line\n"},
		},
		{
			name:  "multiple lines with final newline",
			input: "a\nb\n",
			want:  []string{"a\n", "b\n"},
		},
		{
			name:  "multiple lines without final newline",
			input: "a\nb",
			want:  []string{"a\n", "b"},
		},
		{
			name:  "consecutive newlines",
			input: "a\n\n",
			want:  []string{"a\n", "\n"},
		},
	}

	for _, tt := range tests {
		got := splitLines([]byte(tt.input))
		if len(tt.want) == 0 {
			if got != nil && len(got) != 0 {
				t.Errorf("splitLines(%q) = %q, want nil/empty", tt.name, got)
			}
			continue
		}

		if len(got) != len(tt.want) {
			t.Fatalf("splitLines(%q) len = %d, want %d", tt.name, len(got), len(tt.want))
		}
		for i := range tt.want {
			if string(got[i]) != tt.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.name, i, got[i], tt.want[i])
			}
		}
	}
}

func TestSliceLines_NoLimitsReturnsOriginal(t *testing.T) {
	input := []byte("a\nb\nc\n")
	got := sliceLines(input, 0, 0)
	if !bytes.Equal(got, input) {
		t.Errorf("sliceLines with no limits changed data: got %q, want %q", got, input)
	}
}

func TestSliceLines_EmptyData(t *testing.T) {
	input := []byte{}
	got := sliceLines(input, 5, 5)
	if !bytes.Equal(got, input) {
		t.Errorf("sliceLines on empty data changed data: got %q, want %q", got, input)
	}
}

func TestSliceLines_HeadOnly(t *testing.T) {
	input := []byte("a\nb\nc\n")
	got := sliceLines(input, 2, 0)
	want := []byte("a\nb\n")
	if !bytes.Equal(got, want) {
		t.Errorf("sliceLines head-only = %q, want %q", got, want)
	}
}

func TestSliceLines_TailOnly(t *testing.T) {
	input := []byte("a\nb\nc\n")
	got := sliceLines(input, 0, 2)
	want := []byte("b\nc\n")
	if !bytes.Equal(got, want) {
		t.Errorf("sliceLines tail-only = %q, want %q", got, want)
	}
}

func TestSliceLines_HeadTailWithEllipsis(t *testing.T) {
	input := []byte("a\nb\nc\nd\ne\n")
	got := sliceLines(input, 1, 2)
	want := []byte("a\n... (2 rows skipped)\n" + "d\ne\n")
	if !bytes.Equal(got, want) {
		t.Errorf("sliceLines head+tail with ellipsis = %q, want %q", got, want)
	}
}

func TestSliceLines_HeadCoversAll(t *testing.T) {
	input := []byte("a\nb\nc\n")
	got := sliceLines(input, 5, 0)
	if !bytes.Equal(got, input) {
		t.Errorf("sliceLines head covers all changed data: got %q, want %q", got, input)
	}
}

func TestSliceLines_TailCoversAll(t *testing.T) {
	input := []byte("a\nb\nc\n")
	got := sliceLines(input, 0, 5)
	if !bytes.Equal(got, input) {
		t.Errorf("sliceLines tail covers all changed data: got %q, want %q", got, input)
	}
}

func TestSliceLines_HeadTailCoverAll(t *testing.T) {
	input := []byte("a\nb\nc\n")
	got := sliceLines(input, 1, 2)
	if !bytes.Equal(got, input) {
		t.Errorf("sliceLines head+tail cover all changed data: got %q, want %q", got, input)
	}
}
