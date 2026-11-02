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

	if !reflect.DeepEqual(sections[0].Discover.Includes, []string{"*.go"}) {
		t.Fatalf("Includes = %v, want [*.go]", sections[0].Discover.Includes)
	}
	if !reflect.DeepEqual(sections[0].Discover.Excludes, []string{"*_test.go"}) {
		t.Fatalf("Excludes = %v, want [*_test.go]", sections[0].Discover.Excludes)
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
	if len(sections[0].Discover.Excludes) != 0 {
		t.Fatalf("first Excludes = %v, want empty", sections[0].Discover.Excludes)
	}
	if len(sections[0].Ops) != 1 || sections[0].Ops[0].Action != "FILE" {
		t.Fatalf("first Ops = %+v, want one file op", sections[0].Ops)
	}

	if sections[1].Render.Head != -1 || sections[1].Render.Tail != 0 {
		t.Fatalf("second Render = %+v, want reset defaults before applying exclude", sections[1].Render)
	}
	if !reflect.DeepEqual(sections[1].Discover.Excludes, []string{"vendor/**"}) {
		t.Fatalf("second Excludes = %v, want [vendor/**]", sections[1].Discover.Excludes)
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

func TestApplyInterleaved_FilterFlagsAppend(t *testing.T) {
	s := Section{}

	applyInterleaved(Op{Action: "include", Value: "*.go"}, &s)
	applyInterleaved(Op{Action: "include", Value: "*.md"}, &s)
	applyInterleaved(Op{Action: "exclude", Value: "vendor/**"}, &s)

	if !reflect.DeepEqual(s.Discover.Includes, []string{"*.go", "*.md"}) {
		t.Fatalf("Includes = %v, want [*.go *.md]", s.Discover.Includes)
	}
	if !reflect.DeepEqual(s.Discover.Excludes, []string{"vendor/**"}) {
		t.Fatalf("Excludes = %v, want [vendor/**]", s.Discover.Excludes)
	}
}
