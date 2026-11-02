package cli

import (
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

	sections := parseSections(ops, defaultRender)
	if len(sections) != 1 {
		t.Fatalf("len(sections) = %d, want 1", len(sections))
	}

	if want := []lx.Glob{lx.IncludeGlob("*.go"), lx.ExcludeGlob("*_test.go")}; !reflect.DeepEqual(sections[0].Discover.Globs, want) {
		t.Fatalf("Globs = %v, want %v", sections[0].Discover.Globs, want)
	}
	if len(sections[0].Ops) != 1 || sections[0].Ops[0].Action != "FILE" {
		t.Fatalf("Ops = %+v, want one FILE op", sections[0].Ops)
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

	sections := parseSections(ops, defaultRender)
	if len(sections) != 2 {
		t.Fatalf("len(sections) = %d, want 2", len(sections))
	}

	if sections[0].Render.Head != 7 || sections[0].Render.Tail != 0 {
		t.Fatalf("first Render = %+v, want Head=7 Tail=0", sections[0].Render)
	}
	if len(sections[0].Discover.Globs) != 0 {
		t.Fatalf("first Globs = %v, want empty", sections[0].Discover.Globs)
	}
	if len(sections[0].Ops) != 1 || sections[0].Ops[0].Action != "FILE" {
		t.Fatalf("first Ops = %+v, want one file op", sections[0].Ops)
	}

	if sections[1].Render.Head != -1 || sections[1].Render.Tail != 0 {
		t.Fatalf("second Render = %+v, want reset defaults before applying exclude", sections[1].Render)
	}
	if want := []lx.Glob{lx.ExcludeGlob("vendor/**")}; !reflect.DeepEqual(sections[1].Discover.Globs, want) {
		t.Fatalf("second Globs = %v, want %v", sections[1].Discover.Globs, want)
	}
	if len(sections[1].Ops) != 1 || sections[1].Ops[0].Action != "FILE" {
		t.Fatalf("second Ops = %+v, want one FILE op", sections[1].Ops)
	}
}

func TestApplyInterleaved_RunConfigFlags(t *testing.T) {
	s := Section{Render: lx.RenderOptions{Head: -1, Tail: 0}}

	applyInterleaved(Op{Action: "head", Value: "9"}, &s)
	if s.Render.Head != 9 || s.Render.Tail != 0 {
		t.Fatalf("after head: Render = %+v, want Head=9 Tail=0", s.Render)
	}

	applyInterleaved(Op{Action: "tail", Value: "4"}, &s)
	if s.Render.Head != 0 || s.Render.Tail != 4 {
		t.Fatalf("after tail: Render = %+v, want Head=0 Tail=4", s.Render)
	}

	applyInterleaved(Op{Action: "lines", Value: "5"}, &s)
	if s.Render.Head != 3 || s.Render.Tail != 2 {
		t.Fatalf("after lines: Render = %+v, want Head=3 Tail=2", s.Render)
	}

	applyInterleaved(Op{Action: "line-numbers"}, &s)
	if !s.Render.LineNumbers {
		t.Fatalf("line numbers should be enabled")
	}

	applyInterleaved(Op{Action: "functions"}, &s)
	if !s.Render.SkeletonFunctions {
		t.Fatalf("skeleton functions should be enabled")
	}

	applyInterleaved(Op{Action: "types"}, &s)
	if !s.Render.SkeletonTypes {
		t.Fatalf("skeleton types should be enabled")
	}
}

// -i and -e append to one list in command-line order, since the last glob
// to match a path decides it.
func TestApplyInterleaved_FilterFlagsKeepOrder(t *testing.T) {
	s := Section{}

	applyInterleaved(Op{Action: "exclude", Value: "build/*"}, &s)
	applyInterleaved(Op{Action: "include", Value: "build/keep.js"}, &s)
	applyInterleaved(Op{Action: "exclude", Value: "*.min.js"}, &s)

	want := []lx.Glob{lx.ExcludeGlob("build/*"), lx.IncludeGlob("build/keep.js"), lx.ExcludeGlob("*.min.js")}
	if !reflect.DeepEqual(s.Discover.Globs, want) {
		t.Fatalf("Globs = %v, want %v", s.Discover.Globs, want)
	}
}
