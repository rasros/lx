package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

// -i and -e work as ripgrep's -g glob and -g '!glob': the last one to match
// a path decides it, ahead of .gitignore and hidden-file skipping, and once
// there is an -i, a file must be kept by some glob. They match the path from
// the working directory as the root was named. Every listing here is what
// rg --files lists with the same globs, except where a case says otherwise.
func TestGoldenGlobs(t *testing.T) {
	dir := gf.SetupGlobsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "900_glob_exclude_then_include", args: []string{"-n0", "-e", "build/*", "-i", "build/keep.js", "."}},
		{name: "901_glob_include_then_exclude", args: []string{"-n0", "-i", "build/keep.js", "-e", "build/*", "-i", "b.txt", "."}},
		{name: "902_glob_include_bang_excludes", args: []string{"-n0", "-i", "*.go", "-i", "!src/**", "."}},
		// rg has no glob that keeps a path without also dropping the files
		// no glob keeps; lx's -e '!glob' is that.
		{name: "903_glob_exclude_bang_keeps", args: []string{"-n0", "-e", "ovr/**", "-e", "!ovr/keep.js", "."}},
		{name: "904_glob_include_anchored_at_cwd", args: []string{"-n0", "-i", "build/*.js", "build"}},
		{name: "905_glob_include_overrides_gitignore", args: []string{"-n0", "-i", "*.log", "."}},
		{name: "906_glob_include_overrides_hidden_file", args: []string{"-n0", "-i", "*.go", "."}},
		{name: "907_glob_include_hidden_dir", args: []string{"-n0", "-i", ".hdir", "-i", "*.go", "."}},
		{name: "908_glob_include_gitignored_dir", args: []string{"-n0", "-i", "ign", "-i", "*.go", "."}},
		// A glob matches a directory only as itself, and no later glob keeps
		// anything below an excluded directory, which is not entered.
		{name: "909_glob_dirs_match_only_as_themselves", args: []string{"-n0", "-i", "build", "-e", "build/**", "-i", "build/sub/y.js", "-e", "src", "-i", "src/a.go", "-i", "b.txt", "."}},
		{name: "911_glob_excluded_contents_reincluded", args: []string{"-n0", "-e", "build/**", "-i", "build/keep.js", "."}},
		{name: "913_glob_named_file_not_filtered", args: []string{"-n0", "-e", "a.go", "-i", "zzz", "a.go"}},
		{name: "914_glob_named_dir_not_filtered", args: []string{"-n0", "-e", "ign", "ign"}},
		{name: "915_glob_absolute_root_below_cwd", args: []string{"-n0", "-i", "build/*.js", filepath.Join(dir, "build")}},
		{name: "916_glob_trailing_exclude_matches_last", args: []string{"-n0", "-i", "*.go", ".", "-e", "/a.go"}},
		// -s starts a section with its globs reset, and globs no file follows
		// before the next -s or the end apply to the section before them.
		{name: "917_section_resets_globs", args: []string{"-s", "A", "-n0", "-i", "*.js", "build", "-s", "B", "-n0", "src"}},
		{name: "918_section_trailing_glob_applies_to_last", args: []string{"-s", "A", "-n0", "build", "-s", "B", "-n0", "src", "-e", "*_test.go"}},
		{name: "919_section_glob_before_next_section", args: []string{"-s", "A", "-n0", "src", "-e", "*_test.go", "-s", "B", "-n0", "src"}},
		// Options no file has followed yet carry on into the section a -s
		// starts, past prompts and system context.
		{name: "955_section_options_carried_past_prompt", args: []string{"-n0", "-i", "*.go", "-p", "hi", "-s", "A", "."}},
		{name: "956_section_options_carried_past_system_context", args: []string{"-n0", "-E", "-s", "A", "src"}},
		// Options after files apply back to them whatever comes before the
		// next -s, and a tree with no files to show is reported.
		{name: "959_section_trailing_options_past_prompt", args: []string{"build/x.js", "-l", "-p", "hi", "-s", "X", "b.txt"}},
		{name: "959b_section_trailing_options_past_tree", args: []string{"build/x.js", "-l", "-T", "-s", "X", "b.txt"}},

		// Globs and roots are matched as written, as rg matches them: no
		// cleaning of "./", "//" or "..". An -i that is empty or starts with
		// "#" adds no glob, and one that cannot be parsed is an error.
		{name: "950_glob_dot_slash_as_written", args: []string{"-n0", "-i", "./src/*.go", "-i", "src//*.go", "-i", "b.txt", "."}},
		{name: "951_glob_dot_dot_as_written", args: []string{"-n0", "-i", "src/../a.go", "-i", "b.txt", "."}},
		{name: "952_glob_root_as_written", args: []string{"-n0", "-i", "src/../build/*.js", "-i", "build/sub/*", "src/../build"}},
		{name: "953_glob_comment_and_empty_add_nothing", args: []string{"-n0", "-i", "#b.txt", "-i", "", "-e", "*.go", "-e", "*.js", "."}},
		{name: "954_glob_invalid", args: []string{"-n0", "-i", "src/[ab", "."}},
		{name: "954b_glob_invalid_unopened_brace", args: []string{"-n0", "-i", "a}", "."}},
		{name: "954c_glob_invalid_exclude_as_typed", args: []string{"-n0", "-e", "!{a", "."}},
		{name: "954d_glob_invalid_exclude_hash", args: []string{"-n0", "-e", "#[", "."}},
		// A "]" opening a class is part of it, as in rg.
		{name: "957_glob_class_leading_bracket", args: []string{"-n0", "-i", "[]a]*", "-i", "[!]a]*.go", "."}},
		// An empty -e matches every path, as rg's -g '!' does, so only what a
		// later glob keeps is left.
		{name: "958_glob_empty_exclude_drops_everything", args: []string{"-n0", "-e", "", "-i", "b.txt", "."}},

		// rg takes no paths on stdin; a piped path is kept as a walk of the
		// working directory would keep it.
		{name: "920_glob_stdin_exclude_bang_keeps", args: []string{"-n0", "-e", "ovr/**", "-e", "!ovr/keep.js"}, stdin: "ovr"},
		{name: "921_glob_stdin_last_match_wins", args: []string{"-n0", "-e", "build/*", "-i", "build/keep.js"}, stdin: "build/x.js\nbuild"},
		{name: "922_glob_stdin_excluded_parent_dir", args: []string{"-n0", "-e", "build"}, stdin: "build/keep.js\nbuild/sub\nsrc/a.go"},
		{name: "923_stdin_file_and_its_dir_listed_once", args: []string{"-n0"}, stdin: "build/keep.js\nbuild"},
		{name: "924_glob_stdin_paths_cleaned", args: []string{"-n0", "-i", "src/*.go", "-i", "build/**", "-e", "build/sub", "-e", "build/x.js"}, stdin: "build//sub/y.js\nsrc/../build/x.js\nsrc//a.go\n./build/keep.js"},

		// rg does not expand archives; lx matches an expanded archive as a
		// directory and its entries as the paths below it.
		{name: "930_glob_walked_archive_entries", args: []string{"-n0", "-Z", "-e", "arch/a.zip/src/*", "-i", "arch/a.zip/src/a.go", "."}},
		{name: "931_glob_named_archive_entries", args: []string{"-n0", "-Z", "-i", "arch/a.zip/src/*.go", "arch/a.zip"}},
		{name: "932_glob_include_enters_archive", args: []string{"-n0", "-Z", "-i", "*.go", "."}},
	})
}

// The entries of a URL archive are matched below the URL as written, "//"
// and all, so a glob spelling out an entry's displayed path keeps it.
func TestGoldenGlobsURLArchive(t *testing.T) {
	dir := gf.SetupGlobsFixture(t)
	server := httptest.NewServer(http.FileServer(http.Dir(filepath.Join(dir, "arch"))))
	defer server.Close()
	runTestGolden(t, dir, []goldenTestCase{
		{name: "933_glob_url_archive_entries", args: []string{"-n0", "-Z", "-i", server.URL + "/a.zip/src/*.go", "-i", "notes.txt", server.URL + "/a.zip"}},
	}, server.URL)
}

// Below the working directory, a relative root keeps its path as written,
// so globs anchored at the working directory cannot match below "..", while
// an absolute root has the working directory stripped from the front of the
// paths below it.
func TestGoldenGlobsOutsideCwd(t *testing.T) {
	dir := gf.SetupGlobsFixture(t)
	pkgDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(pkgDir) })
	if err := os.Chdir(filepath.Join(dir, "build")); err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(dir)
	runGoldenTests(t, []goldenTestCase{
		{name: "940_glob_parent_root_as_written", args: []string{"-n0", "-i", "../build/*.js", ".."}},
		{name: "941_glob_parent_root_misses_cwd_glob", args: []string{"-n0", "-i", "build/*.js", "-i", "../a.go", ".."}},
		{name: "942_glob_absolute_root_above_cwd", args: []string{"-n0", "-i", "/x.js", dir}},
		{name: "943_glob_absolute_root_outside_cwd", args: []string{"-n0", "-i", "**/ovr/keep.js", "-i", "ovr/drop.js", filepath.Join(dir, "ovr")}},
		{name: "944_glob_absolute_archive_outside_cwd", args: []string{"-n0", "-Z", "-i", "**/a.zip/src/*.go", "-i", "arch/a.zip/notes.txt", filepath.Join(dir, "arch", "a.zip")}},
		// A piped absolute path is judged as a root named that way: the
		// directories above it are not, and below a directory above the
		// working directory paths are matched relative to it.
		{name: "945_glob_stdin_absolute_dir_above_cwd", args: []string{"-n0", "-i", "sub/*", "-i", "/x.js"}, stdin: dir},
		{name: "946_glob_stdin_absolute_path_parents_not_judged", args: []string{"-n0", "-e", "ovr", "-e", "drop.js"}, stdin: filepath.Join(dir, "ovr", "keep.js") + "\n" + filepath.Join(dir, "ovr", "drop.js")},
	}, pkgDir, dir, canonical)
}
