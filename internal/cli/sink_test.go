package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// entries returns the names in dir, to catch a temporary file left behind.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, de := range des {
		names = append(names, de.Name())
	}
	return names
}

func TestFileSink_AbortKeepsOldFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	sink, err := createFileSink(path, "out.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sink.Write([]byte("partial")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, path); got != "old" {
		t.Errorf("file = %q while writing, want the old content", got)
	}
	sink.Abort()

	if got := readString(t, path); got != "old" {
		t.Errorf("file = %q after Abort, want the old content", got)
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Errorf("dir holds %v after Abort, want only out.md", names)
	}
}

func TestFileSink_CommitReplacesFileKeepingMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(path, link); err != nil {
		t.Skip("symlinks unsupported:", err)
	}

	sink, err := createFileSink(link, "link.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sink.Write([]byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(); err != nil {
		t.Fatal(err)
	}
	sink.Abort() // a no-op after Commit

	if got := readString(t, path); got != "new" {
		t.Errorf("file = %q after Commit, want the new content", got)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink was replaced; want the file it points to replaced")
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want the old file's 0600", info.Mode().Perm())
	}
	if names := entries(t, dir); len(names) != 2 {
		t.Errorf("dir holds %v after Commit, want out.md and link.md", names)
	}
}

func TestFileSink_ErrorsNameTheOutputFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := createFileSink(filepath.Join(dir, "missing", "out.md"), "missing/out.md"); err == nil ||
		err.Error() != "open missing/out.md: no such file or directory" {
		t.Errorf("err = %v, want it to name missing/out.md", err)
	}
	if _, err := createFileSink(dir, "dir"); err == nil || err.Error() != "open dir: is a directory" {
		t.Errorf("err = %v, want it to name dir as a directory", err)
	}
}
