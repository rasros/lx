package cli

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

func TestParseSections_NoSplitBeforeFiles(t *testing.T) {
	defaultRender := lx.RenderOptions{Head: -1, Tail: 0}
	ops := []Op{
		{Action: "include", Value: "*.go", Type: CmdInterleaved},
		{Action: "exclude", Value: "*_test.go", Type: CmdInterleaved},
		{Action: "FILE", Value: "main.go", Type: CmdAction},
	}

	sections := compile(slog.Default(), ops, defaultRender)
	if len(sections) != 1 {
		t.Fatalf("len(sections) = %d, want 1", len(sections))
	}

	if want := []lx.Glob{lx.IncludeGlob("*.go"), lx.ExcludeGlob("*_test.go")}; !reflect.DeepEqual(sections[0].Discover.Globs, want) {
		t.Fatalf("Globs = %v, want %v", sections[0].Discover.Globs, want)
	}
	if !isFiles(sections[0].Items, "main.go") {
		t.Fatalf("Items = %+v, want main.go", sections[0].Items)
	}
}

func TestParseSections_SplitsAndResetsAfterFiles(t *testing.T) {
	defaultRender := lx.RenderOptions{Head: -1, Tail: 0}
	ops := []Op{
		{Action: "head", Value: "7", Type: CmdInterleaved},
		{Action: "FILE", Value: "first.txt", Type: CmdAction},
		{Action: "exclude", Value: "vendor/**", Type: CmdInterleaved},
		{Action: "FILE", Value: "next.txt", Type: CmdAction},
	}

	sections := compile(slog.Default(), ops, defaultRender)
	if len(sections) != 2 {
		t.Fatalf("len(sections) = %d, want 2", len(sections))
	}

	if sections[0].Render.Head != 7 || sections[0].Render.Tail != 0 {
		t.Fatalf("first Render = %+v, want Head=7 Tail=0", sections[0].Render)
	}
	if len(sections[0].Discover.Globs) != 0 {
		t.Fatalf("first Globs = %v, want empty", sections[0].Discover.Globs)
	}
	if !isFiles(sections[0].Items, "first.txt") {
		t.Fatalf("first Items = %+v, want first.txt", sections[0].Items)
	}

	if sections[1].Render.Head != -1 || sections[1].Render.Tail != 0 {
		t.Fatalf("second Render = %+v, want reset defaults before applying exclude", sections[1].Render)
	}
	if want := []lx.Glob{lx.ExcludeGlob("vendor/**")}; !reflect.DeepEqual(sections[1].Discover.Globs, want) {
		t.Fatalf("second Globs = %v, want %v", sections[1].Discover.Globs, want)
	}
	if !isFiles(sections[1].Items, "next.txt") {
		t.Fatalf("second Items = %+v, want next.txt", sections[1].Items)
	}
}

// isFiles reports whether items are file items naming args, in order.
func isFiles(items []item, args ...string) bool {
	if len(items) != len(args) {
		return false
	}
	for i, it := range items {
		f, ok := it.(*fileItem)
		if !ok || f.arg != args[i] {
			return false
		}
	}
	return true
}

// A --tree-only covers the files after it, up to the next --tree-only, and
// leaves their content out; the files before it keep theirs, and each group
// of trees with no files beside it is kept for a warning.
func TestCompile_TreeGroups(t *testing.T) {
	parsed, err := Parse([]string{"a", "-t", "b", "-", "-t", "-t", "-s", "S", "-T"}, definitions)
	if err != nil {
		t.Fatal(err)
	}
	sections := compile(slog.New(slog.DiscardHandler), parsed.Ops, lx.RenderOptions{Head: -1})
	if len(sections) != 2 {
		t.Fatalf("len(sections) = %d, want 2", len(sections))
	}
	items := sections[0].Items
	a, tree, b, in := items[0].(*fileItem), items[1].(*treeItem), items[2].(*fileItem), items[3].(*stdinItem)
	if a.treeOnly || !b.treeOnly || !in.treeOnly {
		t.Errorf("treeOnly a=%v b=%v stdin=%v, want false true true", a.treeOnly, b.treeOnly, in.treeOnly)
	}
	if len(tree.inputs) != 1 || tree.inputs[0] != b {
		t.Errorf("tree inputs = %v, want b", tree.inputs)
	}
	want := []string{"--tree-only", "--tree-only"}
	if got := sections[0].bareTrees; !reflect.DeepEqual(got, want) {
		t.Errorf("first bareTrees = %q, want %q", got, want)
	}
	if got, want := sections[1].bareTrees, []string{"--tree"}; !reflect.DeepEqual(got, want) {
		t.Errorf("second bareTrees = %q, want %q", got, want)
	}
}

func TestApplyInterleaved_RunConfigFlags(t *testing.T) {
	s := Section{Render: lx.RenderOptions{Head: -1, Tail: 0}}

	applyInterleaved(slog.Default(), Op{Action: "head", Value: "9"}, &s)
	if s.Render.Head != 9 || s.Render.Tail != 0 {
		t.Fatalf("after head: Render = %+v, want Head=9 Tail=0", s.Render)
	}

	applyInterleaved(slog.Default(), Op{Action: "tail", Value: "4"}, &s)
	if s.Render.Head != 0 || s.Render.Tail != 4 {
		t.Fatalf("after tail: Render = %+v, want Head=0 Tail=4", s.Render)
	}

	applyInterleaved(slog.Default(), Op{Action: "lines", Value: "5"}, &s)
	if s.Render.Head != 3 || s.Render.Tail != 2 {
		t.Fatalf("after lines: Render = %+v, want Head=3 Tail=2", s.Render)
	}

	applyInterleaved(slog.Default(), Op{Action: "line-numbers"}, &s)
	if !s.Render.LineNumbers {
		t.Fatalf("line numbers should be enabled")
	}

	applyInterleaved(slog.Default(), Op{Action: "functions"}, &s)
	if !s.Render.SkeletonFunctions {
		t.Fatalf("skeleton functions should be enabled")
	}

	applyInterleaved(slog.Default(), Op{Action: "types"}, &s)
	if !s.Render.SkeletonTypes {
		t.Fatalf("skeleton types should be enabled")
	}
}

// -i and -e append to one list in command-line order, since the last glob
// to match a path decides it.
func TestApplyInterleaved_FilterFlagsKeepOrder(t *testing.T) {
	s := Section{}

	applyInterleaved(slog.Default(), Op{Action: "exclude", Value: "build/*"}, &s)
	applyInterleaved(slog.Default(), Op{Action: "include", Value: "build/keep.js"}, &s)
	applyInterleaved(slog.Default(), Op{Action: "exclude", Value: "*.min.js"}, &s)

	want := []lx.Glob{lx.ExcludeGlob("build/*"), lx.IncludeGlob("build/keep.js"), lx.ExcludeGlob("*.min.js")}
	if !reflect.DeepEqual(s.Discover.Globs, want) {
		t.Fatalf("Globs = %v, want %v", s.Discover.Globs, want)
	}
}
