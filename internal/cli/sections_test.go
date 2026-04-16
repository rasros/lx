package cli

import (
	"reflect"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

func TestParseSections_NoSplitBeforeFiles(t *testing.T) {
	defaultRunCfg := lx.RunnerConfig{Head: -1, Tail: 0}
	ops := []Op{
		{Action: "include", Value: "*.go", Type: CmdInterleaved},
		{Action: "exclude", Value: "*_test.go", Type: CmdInterleaved},
		{Action: "FILE", Value: "main.go", Type: CmdAction},
	}

	sections := parseSections(ops, defaultRunCfg)
	if len(sections) != 1 {
		t.Fatalf("len(sections) = %d, want 1", len(sections))
	}

	if !reflect.DeepEqual(sections[0].Includes, []string{"*.go"}) {
		t.Fatalf("Includes = %v, want [*.go]", sections[0].Includes)
	}
	if !reflect.DeepEqual(sections[0].Excludes, []string{"*_test.go"}) {
		t.Fatalf("Excludes = %v, want [*_test.go]", sections[0].Excludes)
	}
	if len(sections[0].Ops) != 1 || sections[0].Ops[0].Action != "FILE" {
		t.Fatalf("Ops = %+v, want one FILE op", sections[0].Ops)
	}
}

func TestParseSections_SplitsAndResetsAfterFiles(t *testing.T) {
	defaultRunCfg := lx.RunnerConfig{Head: -1, Tail: 0}
	ops := []Op{
		{Action: "head", Value: "7", Type: CmdInterleaved},
		{Action: "file", Value: "forced.txt", Type: CmdAction},
		{Action: "exclude", Value: "vendor/**", Type: CmdInterleaved},
		{Action: "FILE", Value: "next.txt", Type: CmdAction},
	}

	sections := parseSections(ops, defaultRunCfg)
	if len(sections) != 2 {
		t.Fatalf("len(sections) = %d, want 2", len(sections))
	}

	if sections[0].RunCfg.Head != 7 || sections[0].RunCfg.Tail != 0 {
		t.Fatalf("first RunCfg = %+v, want Head=7 Tail=0", sections[0].RunCfg)
	}
	if len(sections[0].Excludes) != 0 {
		t.Fatalf("first Excludes = %v, want empty", sections[0].Excludes)
	}
	if len(sections[0].Ops) != 1 || sections[0].Ops[0].Action != "file" {
		t.Fatalf("first Ops = %+v, want one forced file op", sections[0].Ops)
	}

	if sections[1].RunCfg.Head != -1 || sections[1].RunCfg.Tail != 0 {
		t.Fatalf("second RunCfg = %+v, want reset defaults before applying exclude", sections[1].RunCfg)
	}
	if !reflect.DeepEqual(sections[1].Excludes, []string{"vendor/**"}) {
		t.Fatalf("second Excludes = %v, want [vendor/**]", sections[1].Excludes)
	}
	if len(sections[1].Ops) != 1 || sections[1].Ops[0].Action != "FILE" {
		t.Fatalf("second Ops = %+v, want one FILE op", sections[1].Ops)
	}
}

func TestApplyInterleaved_RunConfigFlags(t *testing.T) {
	s := Section{RunCfg: lx.RunnerConfig{Head: -1, Tail: 0}}

	applyInterleaved(Op{Action: "head", Value: "9"}, &s)
	if s.RunCfg.Head != 9 || s.RunCfg.Tail != 0 {
		t.Fatalf("after head: RunCfg = %+v, want Head=9 Tail=0", s.RunCfg)
	}

	applyInterleaved(Op{Action: "tail", Value: "4"}, &s)
	if s.RunCfg.Head != 0 || s.RunCfg.Tail != 4 {
		t.Fatalf("after tail: RunCfg = %+v, want Head=0 Tail=4", s.RunCfg)
	}

	applyInterleaved(Op{Action: "lines", Value: "5"}, &s)
	if s.RunCfg.Head != 3 || s.RunCfg.Tail != 2 {
		t.Fatalf("after lines: RunCfg = %+v, want Head=3 Tail=2", s.RunCfg)
	}

	applyInterleaved(Op{Action: "line-numbers"}, &s)
	if !s.RunCfg.LineNumbers {
		t.Fatalf("line numbers should be enabled")
	}

	applyInterleaved(Op{Action: "functions"}, &s)
	if !s.RunCfg.SkeletonFunctions {
		t.Fatalf("skeleton functions should be enabled")
	}

	applyInterleaved(Op{Action: "types"}, &s)
	if !s.RunCfg.SkeletonTypes {
		t.Fatalf("skeleton types should be enabled")
	}
}

func TestApplyInterleaved_FilterFlagsAppend(t *testing.T) {
	s := Section{}

	applyInterleaved(Op{Action: "include", Value: "*.go"}, &s)
	applyInterleaved(Op{Action: "include", Value: "*.md"}, &s)
	applyInterleaved(Op{Action: "exclude", Value: "vendor/**"}, &s)

	if !reflect.DeepEqual(s.Includes, []string{"*.go", "*.md"}) {
		t.Fatalf("Includes = %v, want [*.go *.md]", s.Includes)
	}
	if !reflect.DeepEqual(s.Excludes, []string{"vendor/**"}) {
		t.Fatalf("Excludes = %v, want [vendor/**]", s.Excludes)
	}
}
