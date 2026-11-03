package main

import (
	"path/filepath"
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

func TestGoldenPromptLib(t *testing.T) {
	dir := gf.SetupPromptLibFixture(t)
	runTestGoldenEnv(t, dir, map[string]string{"LX_PROMPTS_DIR": filepath.Join(dir, "prompts")}, []goldenTestCase{
		{name: "800_promptlib_nested", args: []string{"-P", "go/test", "main.go"}},
		{name: "801_promptlib_basename", args: []string{"-P", "refactor", "main.go"}},
		{name: "802_promptlib_extension_probe", args: []string{"-P", "plan", "main.go"}},
		{name: "803_promptlib_stack", args: []string{"-P", "refactor", "-P", "go/test", "main.go"}},
		{name: "804_promptlib_xml", args: []string{"--xml", "-P", "go/test", "main.go"}},
		{name: "805_promptlib_path_relative", args: []string{"-P", "./prompts/refactor.md", "main.go"}},
		{name: "806_promptlib_not_found", args: []string{"-P", "missing", "main.go"}},
		{name: "807_promptlib_with_section", args: []string{"-P", "go/test", "-s", "Code", "main.go"}},
		{name: "808_promptlib_shallower_wins", args: []string{"-P", "test", "main.go"}},
		{name: "809_promptlib_ambiguous_same_level", args: []string{"-P", "dup", "main.go"}},
		{name: "810_promptlib_shallower_nested_wins", args: []string{"-P", "note", "main.go"}},
		// A bad prompt fails before any input is collected, so the warning
		// collecting bundle.md would give is never reached.
		{name: "811_promptlib_not_found_before_inputs", args: []string{"-o", "bundle.md", "bundle.md", "main.go", "-P", "missing"}, outFile: "bundle.md"},
		// An empty name is a prompt to find, not an empty prompt.
		{name: "812_promptlib_empty_name", args: []string{"-P", "", "main.go"}},
	})
}
