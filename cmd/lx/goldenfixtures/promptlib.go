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

	t.Setenv("LX_PROMPTS_DIR", filepath.Join(dir, "prompts"))

	return dir
}
