package goldenfixtures

import (
	"path/filepath"
	"testing"
)

func SetupPromptLibFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}\n", 0644)
	writeFile(t, dir, "README.md", "# Project\n", 0644)

	writeFile(t, dir, "prompts/go/test.md", "Write table-driven tests for the Greeter type.\n", 0644)
	writeFile(t, dir, "prompts/refactor.md", "Refactor for readability without changing behavior.\n", 0644)
	writeFile(t, dir, "prompts/plan.txt", "Outline a step-by-step plan first.\n", 0644)
	writeFile(t, dir, "prompts/test.md", "Top-level test prompt.\n", 0644)
	writeFile(t, dir, "prompts/a/dup.md", "First duplicate.\n", 0644)
	writeFile(t, dir, "prompts/b/dup.md", "Second duplicate.\n", 0644)
	writeFile(t, dir, "prompts/area/note.md", "Shallow nested note.\n", 0644)
	writeFile(t, dir, "prompts/area/sub/note.md", "Deeper nested note.\n", 0644)

	t.Setenv("LX_PROMPTS_DIR", filepath.Join(dir, "prompts"))

	return dir
}
