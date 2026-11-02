package main

import (
	"path/filepath"
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

func TestGoldenAncestorIgnore(t *testing.T) {
	dir := setupAncestorIgnoreFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "104_ancestor_ignore_subdir", args: []string{"sub"}},
		{name: "105_ancestor_ignore_explicit_dir", args: []string{"dist"}},
		{name: "105b_ancestor_ignore_anchored_dir", args: []string{"build"}},
		{name: "105c_named_ignored_file", args: []string{"sub/a.log"}},
		{name: "105e_named_file_in_ignored_dir", args: []string{"dist/out.js"}},
		{name: "105f_stdin_named_ignored_file", args: []string{}, stdin: "sub/a.log\nsub/main.go"},
		{name: "105i_stdin_include_filter", args: []string{"-i", "*.go"}, stdin: "sub/a.log\nsub/main.go\nsub/deep/util.go\nsub/keep.log"},
		{name: "105j_stdin_exclude_filter", args: []string{"-e", "deep/"}, stdin: "sub/a.log\nsub/main.go\nsub/deep/util.go\nsub/keep.log"},
		{name: "105k_stdin_include_filter_walks_dirs", args: []string{"-i", "*.go"}, stdin: "sub/a.log\nsub/deep\ndist"},
		{name: "105l_stdin_include_filter_single_dir", args: []string{"-i", "*.go"}, stdin: "sub"},
		{name: "105m_stdin_exclude_filter_drops_dir", args: []string{"-e", "deep/"}, stdin: "sub/deep\nsub/main.go"},
		{name: "108_info_exclude_repo_root", args: []string{"."}},
	})
}

func TestGoldenIgnoredParentDir(t *testing.T) {
	dir := setupIgnoredParentDirFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "105g_named_file_in_dir_only_ignored_dir", args: []string{"build/out.js"}},
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
		{name: "109fa_ignored_dir_named", args: []string{"build"}},
		{name: "109fb_ignored_dir_absolute", args: []string{filepath.Join(dir, "build")}},
	})
}

func TestGoldenNegatedIgnoredDirFromSubdir(t *testing.T) {
	dir := setupNegatedIgnoredDirFixture(t)
	runTestGolden(t, filepath.Join(dir, "x"), []goldenTestCase{
		{name: "109g_negated_ignored_dir_parent_walk", args: []string{".."}},
		{name: "109h_negated_ignored_dir_parent_relative_file", args: []string{"../build/keep.js", "../build/out.js"}},
		{name: "109i_negated_ignored_dir_parent_absolute_file", args: []string{filepath.Join(dir, "build", "keep.js")}},
		{name: "109ia_ignored_dir_parent_relative", args: []string{"../build"}},
	}, dir)
}

// The ignore files above the directory lx runs in apply as they do below it:
// .ignore everywhere, .gitignore only inside a git repository.
func TestGoldenParentIgnoreFilesNoRepo(t *testing.T) {
	dir := gf.SetupParentIgnoreFilesFixture(t, false)
	runTestGolden(t, filepath.Join(dir, "src"), []goldenTestCase{
		{name: "109j_no_repo_walk_cwd", args: []string{"."}},
		{name: "109k_no_repo_walk_parent", args: []string{".."}},
	}, dir)
}

func TestGoldenParentIgnoreFilesRepo(t *testing.T) {
	dir := gf.SetupParentIgnoreFilesFixture(t, true)
	runTestGolden(t, filepath.Join(dir, "src"), []goldenTestCase{
		{name: "109l_repo_walk_cwd", args: []string{"."}},
		{name: "109m_repo_walk_parent", args: []string{".."}},
	}, dir)
}

// A nested repository or linked worktree inside an ignored directory is
// skipped by a walk of the enclosing repository, but judged by its own rules
// when walked or named directly.
func TestGoldenNestedReposInIgnoredDirs(t *testing.T) {
	dir := gf.SetupNestedReposInIgnoredDirsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109n_ignored_nested_repo_outer_walk", args: []string{"."}},
		{name: "109o_ignored_nested_repo_named_dir", args: []string{"build/inner"}},
		{name: "109p_ignored_nested_repo_named_files", args: []string{"build/inner/i.go", "build/inner/i.log"}},
		{name: "109q_worktree_in_ignored_dir_named_dir", args: []string{".worktrees/wt"}},
	})
}

func TestGoldenNestedRepoInIgnoredDirFromInside(t *testing.T) {
	dir := gf.SetupNestedReposInIgnoredDirsFixture(t)
	runTestGolden(t, filepath.Join(dir, "build", "inner"), []goldenTestCase{
		{name: "109r_ignored_nested_repo_from_inside", args: []string{"."}},
	}, dir)
}

func TestGoldenWorktreeInIgnoredDirFromInside(t *testing.T) {
	dir := gf.SetupNestedReposInIgnoredDirsFixture(t)
	runTestGolden(t, filepath.Join(dir, ".worktrees", "wt"), []goldenTestCase{
		{name: "109s_worktree_in_ignored_dir_from_inside", args: []string{"."}},
	}, dir)
}

func TestGoldenSymlinkedRoots(t *testing.T) {
	dir := gf.SetupSymlinkedRootsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109t_symlinked_root_into_repo", args: []string{"srclink"}},
		{name: "109u_symlinked_root_out_of_repo", args: []string{"repo/lnk"}},
	})
}

func TestGoldenIgnoreSyntax(t *testing.T) {
	dir := gf.SetupIgnoreSyntaxFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109v_negation_includes_hidden_and_trailing_spaces", args: []string{"."}},
		{name: "109w_named_hidden_files", args: []string{".kunitconfig", ".hidden"}},
	})
}

func TestGoldenStrayGitDirAbove(t *testing.T) {
	dir := setupStrayGitDirAboveFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "109c_stray_git_dir_above_is_not_a_repo", args: []string{"."}},
	}, filepath.Dir(dir))
}
