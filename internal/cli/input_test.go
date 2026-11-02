package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestReadFilenamesFromStdin_Piped(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	input := "file1.txt\n\nfile2.txt \n  \nfile3\n"
	go func() {
		defer w.Close()
		if _, err := w.Write([]byte(input)); err != nil {
			t.Error(err)
		}
	}()

	got, _, err := readFilenamesFromStdin(r, false)
	if err != nil {
		t.Fatalf("readFilenamesFromStdin error: %v", err)
	}

	want := []string{"file1.txt", "file2.txt", "file3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readFilenamesFromStdin = %v, want %v", got, want)
	}
}

func TestReadFilenamesFromStdin_Null(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	input := []byte("file 1.txt\x00file\n2.txt\x00file3\x00")

	go func() {
		defer w.Close()
		if _, err := w.Write(input); err != nil {
			t.Error(err)
		}
	}()

	got, _, err := readFilenamesFromStdin(r, true)
	if err != nil {
		t.Fatalf("readFilenamesFromStdin(true) error: %v", err)
	}

	want := []string{"file 1.txt", "file\n2.txt", "file3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readFilenamesFromStdin(true) = %q, want %q", got, want)
	}
}
