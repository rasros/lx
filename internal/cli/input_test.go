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
		w.Close()
	}()

	os.Stdin = r

	input := "file1.txt\n\nfile2.txt \n  \nfile3\n"
	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	w.Close()

	got, err := readFilenamesFromStdin()
	if err != nil {
		t.Fatalf("readFilenamesFromStdin error: %v", err)
	}

	want := []string{"file1.txt", "file2.txt", "file3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readFilenamesFromStdin = %v, want %v", got, want)
	}
}
