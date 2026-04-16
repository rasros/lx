package main

import "testing"

func TestGoldenTree(t *testing.T) {
	dir := setupTreeFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		// Tree-only: just the tree, no file content
		{name: "700_tree_only_subdir", args: []string{"-t", "src"}},
		{name: "701_tree_only_dot", args: []string{"-t", "."}},
		{name: "702_tree_only_single_file", args: []string{"-t", "main.go"}},
		{name: "703_tree_only_multiple_dirs", args: []string{"-t", "src", "lib"}},

		// Tree + files: tree at op position, content also included
		{name: "710_tree_before_files", args: []string{"-T", "src"}},
		{name: "711_tree_after_files", args: []string{"src", "-T"}},
		{name: "712_tree_multiple_dirs_with_content", args: []string{"-T", "src", "lib"}},

		// Default directory (no explicit path)
		{name: "713_tree_only_default", args: []string{"-t"}},

		// Group boundaries: section and interleaved options split groups
		{name: "720_tree_section_boundary", args: []string{"-T", "src", "-s", "Lib", "lib"}},
		{name: "721_tree_interleaved_boundary", args: []string{"-T", "src", "-i", "*.go", "lib"}},
		{name: "722_tree_per_section", args: []string{"-t", "src", "-s", "Lib", "-t", "lib"}},

		// Prompts do not create boundaries
		{name: "723_tree_prompt_no_boundary", args: []string{"-T", "src", "-p", "note", "lib"}},

		// Filtering interacts with tree
		{name: "730_tree_with_include", args: []string{"-i", "*.go", "-t", "."}},
		{name: "731_tree_with_exclude", args: []string{"-e", "*_test.go", "-t", "."}},

		// Formats
		{name: "740_tree_xml", args: []string{"--xml", "-t", "src"}},
		{name: "741_tree_html", args: []string{"--html", "-t", "src"}},
		{name: "742_tree_xml_with_content", args: []string{"--xml", "-T", "src"}},
	})
}
