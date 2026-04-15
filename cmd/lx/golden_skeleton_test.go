package main

import "testing"

func TestGoldenSkeleton(t *testing.T) {
	dir := setupSkeletonFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "200_skeleton_go_functions", args: []string{"-F", "skeleton/main.go"}},
		{name: "201_skeleton_go_structs", args: []string{"-T", "skeleton/main.go"}},
		{name: "202_skeleton_go_both", args: []string{"-F", "-T", "skeleton/main.go"}},
		{name: "203_skeleton_python_functions", args: []string{"-F", "skeleton/main.py"}},
		{name: "204_skeleton_python_structs", args: []string{"-T", "skeleton/main.py"}},
		{name: "205_skeleton_python_both", args: []string{"-F", "-T", "skeleton/main.py"}},
		{name: "206_skeleton_c_functions", args: []string{"-F", "skeleton/main.c"}},
		{name: "207_skeleton_c_structs", args: []string{"-T", "skeleton/main.c"}},
		{name: "208_skeleton_c_both", args: []string{"-F", "-T", "skeleton/main.c"}},
		{name: "209_skeleton_cpp_both", args: []string{"-F", "-T", "skeleton/main.cpp"}},
		{name: "210_skeleton_java_structs", args: []string{"-T", "skeleton/Main.java"}},
		{name: "211_skeleton_java_both", args: []string{"-F", "-T", "skeleton/Main.java"}},
		{name: "212_skeleton_interleaved_reset", args: []string{"-F", "-T", "skeleton/main.py", "--reset-skeleton", "skeleton/main.py"}},
		{name: "213_skeleton_toggle_off_functions", args: []string{"-F", "-T", "skeleton/main.go", "--no-functions", "skeleton/main.go"}},
		{name: "214_skeleton_rust_both", args: []string{"-F", "-T", "skeleton/main.rs"}},
		{name: "215_skeleton_typescript_both", args: []string{"-F", "-T", "skeleton/main.ts"}},
		{name: "216_skeleton_kotlin_both", args: []string{"-F", "-T", "skeleton/main.kt"}},
		{name: "217_skeleton_ruby_both", args: []string{"-F", "-T", "skeleton/main.rb"}},
	})
}

func TestGoldenSkeletonSlicing(t *testing.T) {
	dir := setupSkeletonFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "220_skeleton_lines_go_both", args: []string{"-F", "-T", "--lines", "3", "skeleton/main.go"}},
		{name: "221_skeleton_head_go_both", args: []string{"-F", "-T", "--head", "2", "skeleton/main.go"}},
		{name: "222_skeleton_tail_go_both", args: []string{"-F", "-T", "--tail", "2", "skeleton/main.go"}},
		{name: "223_skeleton_head_tail_go_both", args: []string{"-F", "-T", "--head", "2", "--tail", "2", "skeleton/main.go"}},
		{name: "224_skeleton_lines_python_both", args: []string{"-F", "-T", "--lines", "2", "skeleton/main.py"}},
		{name: "225_skeleton_head_cpp_both", args: []string{"-F", "-T", "--head", "3", "skeleton/main.cpp"}},
		{name: "226_skeleton_tail_java_both", args: []string{"-F", "-T", "--tail", "2", "skeleton/Main.java"}},
		{name: "227_skeleton_lines_after_reset", args: []string{"-F", "-T", "--reset-skeleton", "--lines", "2", "skeleton/main.py"}},
		{name: "228_skeleton_progressive_lines_head_tail", args: []string{"-F", "-T", "--lines", "3", "skeleton/main.go", "--head", "1", "skeleton/main.go", "--tail", "1", "skeleton/main.go"}},
	})
}
