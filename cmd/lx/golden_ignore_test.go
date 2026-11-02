package main

import (
	"path/filepath"
	"testing"
)

func TestGoldenAncestorIgnore(t *testing.T) {
	dir := setupAncestorIgnoreFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "104_ancestor_ignore_subdir", args: []string{"sub"}},
		{name: "105_ancestor_ignore_explicit_dir", args: []string{"dist"}},
		{name: "105b_ancestor_ignore_anchored_dir", args: []string{"build"}},
		{name: "105c_force_ignored_file", args: []string{"-f", "sub/a.log"}},
		{name: "105d_force_ignored_dir", args: []string{"-f", "dist"}},
		{name: "105e_warn_named_file_in_ignored_dir", args: []string{"dist/out.js"}},
		{name: "105f_stdin_ignored_file_quiet", args: []string{}, stdin: "sub/a.log\nsub/main.go"},
	})
}

func TestGoldenAncestorIgnoreFromSubdir(t *testing.T) {
	dir := setupAncestorIgnoreFixture(t)
	runTestGolden(t, filepath.Join(dir, "sub"), []goldenTestCase{
		{name: "106_ancestor_ignore_cwd_subdir", args: []string{"."}},
		{name: "107_ancestor_ignore_nested_dir", args: []string{"deep"}},
	}, dir)
}
