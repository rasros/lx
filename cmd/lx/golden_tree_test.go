package main

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenTree(t *testing.T) {
	pkgDir, _ := os.Getwd()
	dir := setupTreeFixture(t)
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer server.Close()

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
		{name: "743_tree_content_line_numbers", args: []string{"-l", "-T", "src"}},
		{name: "744_tree_content_skeleton_functions", args: []string{"-u", "-T", "src"}},

		// URL tree parity
		{name: "745_tree_only_url", args: []string{"-t", server.URL + "/main.go"}},
		{name: "746_tree_url_with_content", args: []string{"-T", server.URL + "/main.go"}},
	}, server.URL)

	if err := os.Chdir(pkgDir); err != nil {
		t.Fatal(err)
	}

	createZipFixture(t, filepath.Join(dir, "archive.zip"), map[string]string{
		"hello.txt":       "Hello from archive!\n",
		"nested/world.go": "package nested\n",
		".hidden_in_zip":  "hidden inside zip\n",
	})
	runTestGolden(t, dir, []goldenTestCase{
		{name: "747_tree_archive_include_go_only", args: []string{"-Z", "-i", "*.go", "-t", "archive.zip"}},
		{name: "748_tree_archive_include_go_with_content", args: []string{"-Z", "-i", "*.go", "-T", "archive.zip"}},
	})
}

func createZipFixture(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip fixture: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip fixture: %v", err)
	}
}
