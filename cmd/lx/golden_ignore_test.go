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
		{name: "108_info_exclude_repo_root", args: []string{"."}},
	})
}

func TestGoldenIgnoredParentDir(t *testing.T) {
	dir := setupIgnoredParentDirFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "105g_warn_named_file_in_dir_only_ignored_dir", args: []string{"build/out.js"}},
		{name: "105h_force_named_file_in_dir_only_ignored_dir", args: []string{"-f", "build/out.js"}},
	})
}

func TestGoldenAncestorIgnoreFromSubdir(t *testing.T) {
	dir := setupAncestorIgnoreFixture(t)
	runTestGolden(t, filepath.Join(dir, "sub"), []goldenTestCase{
		{name: "106_ancestor_ignore_cwd_subdir", args: []string{"."}},
		{name: "107_ancestor_ignore_nested_dir", args: []string{"deep"}},
	}, dir)
}

func TestGoldenWorktreeInfoExclude(t *testing.T) {
	dir := setupWorktreeExcludeFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109_info_exclude_linked_worktree", args: []string{"."}},
	}, filepath.Dir(dir))
}

func TestGoldenStrayGitDir(t *testing.T) {
	dir := setupStrayGitDirFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109b_stray_git_dirs_keep_repo_rules", args: []string{"."}},
	})
}

// Every way of naming a file below an ignored directory agrees with a walk:
// a re-included file is kept and its sibling skipped, named relative to the
// root, by an absolute path, or by a ../ path from a subdirectory.
func TestGoldenNegatedIgnoredDir(t *testing.T) {
	dir := setupNegatedIgnoredDirFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109d_negated_ignored_dir_walk", args: []string{"."}},
		{name: "109e_negated_ignored_dir_named_file", args: []string{"build/keep.js", "build/out.js"}},
		{name: "109f_negated_ignored_dir_absolute_file", args: []string{filepath.Join(dir, "build", "keep.js"), filepath.Join(dir, "build", "out.js")}},
	})
}

func TestGoldenNegatedIgnoredDirFromSubdir(t *testing.T) {
	dir := setupNegatedIgnoredDirFixture(t)
	runTestGolden(t, filepath.Join(dir, "x"), []goldenTestCase{
		{name: "109g_negated_ignored_dir_parent_walk", args: []string{".."}},
		{name: "109h_negated_ignored_dir_parent_relative_file", args: []string{"../build/keep.js", "../build/out.js"}},
		{name: "109i_negated_ignored_dir_parent_absolute_file", args: []string{filepath.Join(dir, "build", "keep.js")}},
	}, dir)
}

func TestGoldenStrayGitDirAbove(t *testing.T) {
	dir := setupStrayGitDirAboveFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109c_stray_git_dir_above_is_not_a_repo", args: []string{"."}},
	}, filepath.Dir(dir))
}
