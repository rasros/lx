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
		{name: "229_skeleton_csharp_structs", args: []string{"-T", "skeleton/main.cs"}},
		{name: "230_skeleton_csharp_both", args: []string{"-F", "-T", "skeleton/main.cs"}},
		{name: "231_skeleton_swift_structs", args: []string{"-T", "skeleton/main.swift"}},
		{name: "232_skeleton_swift_both", args: []string{"-F", "-T", "skeleton/main.swift"}},
		{name: "233_skeleton_scala_structs", args: []string{"-T", "skeleton/main.scala"}},
		{name: "234_skeleton_scala_both", args: []string{"-F", "-T", "skeleton/main.scala"}},
		{name: "235_skeleton_php_structs", args: []string{"-T", "skeleton/main.php"}},
		{name: "236_skeleton_php_both", args: []string{"-F", "-T", "skeleton/main.php"}},
		{name: "237_skeleton_javascript_both", args: []string{"-F", "-T", "skeleton/main.js"}},
		{name: "238_skeleton_jsx_both", args: []string{"-F", "-T", "skeleton/main.jsx"}},
		{name: "239_skeleton_tsx_both", args: []string{"-F", "-T", "skeleton/main.tsx"}},
		{name: "240_skeleton_toggle_off_structs", args: []string{"-F", "-T", "skeleton/main.go", "--no-structs", "skeleton/main.go"}},
		{name: "241_skeleton_java_functions_only", args: []string{"-F", "skeleton/Main.java"}},
		{name: "242_skeleton_csharp_functions_only", args: []string{"-F", "skeleton/main.cs"}},
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
		{name: "243_skeleton_head_swift_both", args: []string{"-F", "-T", "--head", "2", "skeleton/main.swift"}},
		{name: "244_skeleton_tail_scala_both", args: []string{"-F", "-T", "--tail", "2", "skeleton/main.scala"}},
		{name: "245_skeleton_lines_php_both", args: []string{"-F", "-T", "--lines", "3", "skeleton/main.php"}},
		{name: "246_skeleton_lines_jsx_both", args: []string{"-F", "-T", "--lines", "2", "skeleton/main.jsx"}},
		{name: "247_skeleton_head_tsx_both", args: []string{"-F", "-T", "--head", "3", "skeleton/main.tsx"}},
	})
}
