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
	})
}

func TestGoldenAncestorIgnoreFromSubdir(t *testing.T) {
	dir := setupAncestorIgnoreFixture(t)
	runTestGolden(t, filepath.Join(dir, "sub"), []goldenTestCase{
		{name: "106_ancestor_ignore_cwd_subdir", args: []string{"."}},
		{name: "107_ancestor_ignore_nested_dir", args: []string{"deep"}},
	}, dir)
}
