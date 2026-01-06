package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestReadFilenamesFromStdin_Piped(t *testing.T) {
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdin = origStdin
		r.Close()
		// w is closed explicitly below
	}()

	os.Stdin = r

	input := "file1.txt\n\nfile2.txt \n  \nfile3\n"
	go func() {
		defer w.Close()
		if _, err := w.Write([]byte(input)); err != nil {
			t.Error(err)
		}
	}()

	// Pass false for standard newline splitting
	got, err := readFilenamesFromStdin(false)
	if err != nil {
		t.Fatalf("readFilenamesFromStdin error: %v", err)
	}

	want := []string{"file1.txt", "file2.txt", "file3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readFilenamesFromStdin = %v, want %v", got, want)
	}
}

func TestReadFilenamesFromStdin_Null(t *testing.T) {
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdin = origStdin
		r.Close()
	}()

	os.Stdin = r

	// Simulate output from 'find . -print0' or similar
	// Note: Spaces are preserved in filenames, only \0 splits
	input := []byte("file 1.txt\x00file\n2.txt\x00file3\x00")

	go func() {
		defer w.Close()
		if _, err := w.Write(input); err != nil {
			t.Error(err)
		}
	}()

	// Pass true for null byte splitting
	got, err := readFilenamesFromStdin(true)
	if err != nil {
		t.Fatalf("readFilenamesFromStdin(true) error: %v", err)
	}

	want := []string{"file 1.txt", "file\n2.txt", "file3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readFilenamesFromStdin(true) = %q, want %q", got, want)
	}
}
