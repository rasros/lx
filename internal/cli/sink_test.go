package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// commit writes content to the file at path through a sink.
func commit(t *testing.T, path, content string) {
	t.Helper()
	sink, err := createFileSink(path, filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Abort()
	if _, err := sink.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(); err != nil {
		t.Fatal(err)
	}
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
	if names := entries(t, dir); !slices.Equal(names, []string{"out.md"}) {
		t.Errorf("dir holds %v after Abort, want only out.md", names)
	}
}

func TestFileSink_CommitReplacesFileKeepingMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := os.WriteFile(path, []byte("old content"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}

	commit(t, path, "new")

	if got := readString(t, path); got != "new" {
		t.Errorf("file = %q after Commit, want the new content", got)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want the old file's 0640", info.Mode().Perm())
	}
	if names := entries(t, dir); !slices.Equal(names, []string{"out.md"}) {
		t.Errorf("dir holds %v after Commit, want only out.md", names)
	}
}

// A symlink is written through, a dangling one too, as os.Create does.
func TestFileSink_WritesThroughSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	dangling := filepath.Join(dir, "dangling.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := os.Symlink("created.md", dangling); err != nil {
		t.Fatal(err)
	}

	commit(t, link, "via link")
	commit(t, dangling, "via dangling link")

	if got := readString(t, target); got != "via link" {
		t.Errorf("target = %q, want what was written through the link", got)
	}
	if got := readString(t, filepath.Join(dir, "created.md")); got != "via dangling link" {
		t.Errorf("created.md = %q, want what was written through the dangling link", got)
	}
	for _, l := range []string{link, dangling} {
		if info, err := os.Lstat(l); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s was replaced; want it kept as a symlink", filepath.Base(l))
		}
	}
}

// A name as long as a file name may be still works: the temporary file has a
// short name of its own.
func TestFileSink_LongName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, strings.Repeat("n", 250)+".md")
	commit(t, path, "first")
	commit(t, path, "second")
	if got := readString(t, path); got != "second" {
		t.Errorf("file = %q, want the second write", got)
	}
}

// A bad path fails up front, naming the path as given, without creating
// anything.
func TestFileSink_ChecksThePathUpFront(t *testing.T) {
	dir := t.TempDir()
	if _, err := createFileSink(filepath.Join(dir, "missing", "out.md"), "missing/out.md"); err == nil ||
		err.Error() != "open missing/out.md: no such file or directory" {
		t.Errorf("err = %v, want it to name missing/out.md", err)
	}
	if _, err := createFileSink(dir, "dir"); err == nil || err.Error() != "open dir: is a directory" {
		t.Errorf("err = %v, want it to name dir as a directory", err)
	}
	if names := entries(t, dir); len(names) != 0 {
		t.Errorf("dir holds %v, want nothing created", names)
	}
}
