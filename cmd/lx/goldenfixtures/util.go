package goldenfixtures

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setupMockConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lx"), 0755)
	os.WriteFile(filepath.Join(dir, "lx", "ignore"), []byte(""), 0644)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func writeFile(t *testing.T, dir, path, content string, perm os.FileMode) {
	t.Helper()
	fp := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(fp), err)
	}
	if err := os.WriteFile(fp, []byte(content), perm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func makeSymlink(dir, target, name string) {
	fp := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(fp), 0755)
	_ = os.Symlink(filepath.Join(dir, target), fp)
}

func makeSymlinkRaw(dir, target, name string) {
	fp := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(fp), 0755)
	_ = os.Symlink(target, fp)
}

func buildSymlinksDir(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "links/safe_target/recursion.txt", "I am safe", 0644)
	writeFile(t, dir, "links/.hidden_link.txt", "i am hidden in links", 0644)
	makeSymlink(dir, "main.go", "links/link_to_main.go")
	makeSymlink(dir, "pkg", "links/link_to_pkg")
	makeSymlinkRaw(dir, "does_not_exist", "links/broken_link")
	makeSymlink(dir, "links/safe_target", "links/loop")
	writeFile(t, dir, "links/cycle_a/visible.txt", "a", 0644)
	writeFile(t, dir, "links/cycle_b/visible.txt", "b", 0644)
	makeSymlinkRaw(dir, "../cycle_b", "links/cycle_a/to_b")
	makeSymlinkRaw(dir, "../cycle_a", "links/cycle_b/to_a")
}

func buildLargeFile(t *testing.T, dir, path string) {
	t.Helper()
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		sb.WriteString("Line ")
		sb.WriteString(strings.Repeat("x", 10))
		sb.WriteString("\n")
	}
	writeFile(t, dir, path, sb.String(), 0644)
}

func readFixtureFile(t *testing.T, relPath string) []byte {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve fixture path: runtime caller unavailable")
	}
	fixturePath := filepath.Join(filepath.Dir(thisFile), "..", "testdata", relPath)
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", relPath, err)
	}
	return data
}
