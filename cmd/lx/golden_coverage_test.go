package main

import (
	"path/filepath"
	"testing"
)

// --output redirects the bundle to a file, which moves stats to stdout and
// makes the output file itself a walk candidate.
func TestGoldenOutputFile(t *testing.T) {
	dir := setupWalkFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "170_output_file_stats_to_stdout", args: []string{"--stats", "-o", "bundle.md", "main.go"}},
		{name: "171_output_file_excluded_from_own_walk", args: []string{"--stats", "-o", "bundle.md", "."}},
		{name: "172_output_file_quiet", args: []string{"-q", "-o", "bundle.md", "main.go"}},
	})
}

// bare deliberately emits content only: trees, prompts, sections and error rows
// all render empty.
func TestGoldenBareSuppression(t *testing.T) {
	dir := setupSectionsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "173_bare_drops_tree", args: []string{"--bare", "-T", "doc"}},
		{name: "174_bare_drops_prompt", args: []string{"--bare", "-p", "Explain this", "main.go"}},
		{name: "175_bare_drops_section", args: []string{"--bare", "-s", "Heading", "main.go"}},
		{name: "176_bare_drops_error_row", args: []string{"--bare", "main.go", "missing.go"}},
		{name: "177_bare_tree_only_is_empty", args: []string{"--bare", "-t", "doc"}},
	})
}

// Format, verbosity and prompt extensions are all reachable from config rather
// than flags; only the flag paths were covered.
func TestGoldenConfigDrivenBehaviour(t *testing.T) {
	dir := setupFormattingFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "178_config_sets_output_format", args: []string{"-y", "configs/xml_format.yaml", "main.go"}},
		{name: "179_flag_overrides_config_format", args: []string{"-y", "configs/xml_format.yaml", "--md", "main.go"}},
		{name: "180_config_sets_verbosity", args: []string{"-y", "configs/verbose.yaml", "main.go"}},
	})
}

func TestGoldenPromptsDir(t *testing.T) {
	dir := setupPromptLibFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "181_list_prompts", args: []string{"--list-prompts"}},
		{name: "182_prompts_dir_override", args: []string{
			"--prompts-dir", filepath.Join(dir, "prompts", "area"), "-P", "note", "main.go",
		}},
		{name: "183_prompts_dir_override_nested", args: []string{
			"--prompts-dir", filepath.Join(dir, "prompts", "area"), "-P", "sub/note", "main.go",
		}},
	})
}

// Help is the flag table; it changes whenever a flag is added.
func TestGoldenHelp(t *testing.T) {
	dir := setupWalkFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "184_help_short", args: []string{"-h"}},
		{name: "185_help_long", args: []string{"--help"}},
	})
}

// A standalone .gz has no entry list; it is exposed as a single virtual entry.
func TestGoldenSingleFileCompression(t *testing.T) {
	dir := setupArchiveFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "186_expand_single_file_gz", args: []string{"-Z", "notes.txt.gz"}},
		{name: "187_single_file_gz_without_flag", args: []string{"notes.txt.gz"}},
		{name: "188_expand_single_file_gz_tree", args: []string{"-Z", "-t", "notes.txt.gz"}},
	})
}
