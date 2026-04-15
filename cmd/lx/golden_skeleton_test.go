package main

import "testing"

func TestGoldenSkeleton(t *testing.T) {
	dir := setupSkeletonFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		// 200-209: Go
		{name: "200_skeleton_go_functions", args: []string{"-F", "skeleton/main.go"}},
		{name: "201_skeleton_go_structs", args: []string{"-T", "skeleton/main.go"}},
		{name: "202_skeleton_go_both", args: []string{"-F", "-T", "skeleton/main.go"}},

		// 210-219: Python
		{name: "210_skeleton_python_functions", args: []string{"-F", "skeleton/main.py"}},
		{name: "211_skeleton_python_structs", args: []string{"-T", "skeleton/main.py"}},
		{name: "212_skeleton_python_both", args: []string{"-F", "-T", "skeleton/main.py"}},

		// 220-229: C-family
		{name: "220_skeleton_c_functions", args: []string{"-F", "skeleton/main.c"}},
		{name: "221_skeleton_c_structs", args: []string{"-T", "skeleton/main.c"}},
		{name: "222_skeleton_c_both", args: []string{"-F", "-T", "skeleton/main.c"}},
		{name: "223_skeleton_cpp_both", args: []string{"-F", "-T", "skeleton/main.cpp"}},

		// 230-239: Java
		{name: "230_skeleton_java_structs", args: []string{"-T", "skeleton/Main.java"}},
		{name: "231_skeleton_java_both", args: []string{"-F", "-T", "skeleton/Main.java"}},
		{name: "232_skeleton_java_functions_only", args: []string{"-F", "skeleton/Main.java"}},

		// 240-259: Rust + JS/TS family
		{name: "240_skeleton_rust_both", args: []string{"-F", "-T", "skeleton/main.rs"}},
		{name: "250_skeleton_typescript_both", args: []string{"-F", "-T", "skeleton/main.ts"}},
		{name: "251_skeleton_javascript_both", args: []string{"-F", "-T", "skeleton/main.js"}},
		{name: "252_skeleton_jsx_both", args: []string{"-F", "-T", "skeleton/main.jsx"}},
		{name: "253_skeleton_tsx_both", args: []string{"-F", "-T", "skeleton/main.tsx"}},

		// 260-269: Kotlin / Ruby
		{name: "260_skeleton_kotlin_both", args: []string{"-F", "-T", "skeleton/main.kt"}},
		{name: "261_skeleton_ruby_both", args: []string{"-F", "-T", "skeleton/main.rb"}},

		// 270-279: C#
		{name: "270_skeleton_csharp_structs", args: []string{"-T", "skeleton/main.cs"}},
		{name: "271_skeleton_csharp_both", args: []string{"-F", "-T", "skeleton/main.cs"}},
		{name: "272_skeleton_csharp_functions_only", args: []string{"-F", "skeleton/main.cs"}},

		// 280-309: Swift / Scala / PHP
		{name: "280_skeleton_swift_structs", args: []string{"-T", "skeleton/main.swift"}},
		{name: "281_skeleton_swift_both", args: []string{"-F", "-T", "skeleton/main.swift"}},
		{name: "290_skeleton_scala_structs", args: []string{"-T", "skeleton/main.scala"}},
		{name: "291_skeleton_scala_both", args: []string{"-F", "-T", "skeleton/main.scala"}},
		{name: "300_skeleton_php_structs", args: []string{"-T", "skeleton/main.php"}},
		{name: "301_skeleton_php_both", args: []string{"-F", "-T", "skeleton/main.php"}},

		// 320-329: Interleaved state toggles
		{name: "320_skeleton_interleaved_reset", args: []string{"-F", "-T", "skeleton/main.py", "--reset-skeleton", "skeleton/main.py"}},
		{name: "321_skeleton_toggle_off_functions", args: []string{"-F", "-T", "skeleton/main.go", "--no-functions", "skeleton/main.go"}},
		{name: "322_skeleton_toggle_off_structs", args: []string{"-F", "-T", "skeleton/main.go", "--no-structs", "skeleton/main.go"}},
	})
}

func TestGoldenSkeletonSlicing(t *testing.T) {
	dir := setupSkeletonFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		// 400-409: Go slicing
		{name: "400_skeleton_lines_go_both", args: []string{"-F", "-T", "--lines", "3", "skeleton/main.go"}},
		{name: "401_skeleton_head_go_both", args: []string{"-F", "-T", "--head", "2", "skeleton/main.go"}},
		{name: "402_skeleton_tail_go_both", args: []string{"-F", "-T", "--tail", "2", "skeleton/main.go"}},
		{name: "403_skeleton_head_tail_go_both", args: []string{"-F", "-T", "--head", "2", "--tail", "2", "skeleton/main.go"}},
		{name: "404_skeleton_progressive_lines_head_tail", args: []string{"-F", "-T", "--lines", "3", "skeleton/main.go", "--head", "1", "skeleton/main.go", "--tail", "1", "skeleton/main.go"}},

		// 410-419: Existing language slicing parity
		{name: "410_skeleton_lines_python_both", args: []string{"-F", "-T", "--lines", "2", "skeleton/main.py"}},
		{name: "411_skeleton_head_cpp_both", args: []string{"-F", "-T", "--head", "3", "skeleton/main.cpp"}},
		{name: "412_skeleton_tail_java_both", args: []string{"-F", "-T", "--tail", "2", "skeleton/Main.java"}},

		// 420-429: Interleaved state with slicing
		{name: "420_skeleton_lines_after_reset", args: []string{"-F", "-T", "--reset-skeleton", "--lines", "2", "skeleton/main.py"}},

		// 430-439: New language slicing
		{name: "430_skeleton_head_swift_both", args: []string{"-F", "-T", "--head", "2", "skeleton/main.swift"}},
		{name: "431_skeleton_tail_scala_both", args: []string{"-F", "-T", "--tail", "2", "skeleton/main.scala"}},
		{name: "432_skeleton_lines_php_both", args: []string{"-F", "-T", "--lines", "3", "skeleton/main.php"}},
		{name: "433_skeleton_lines_jsx_both", args: []string{"-F", "-T", "--lines", "2", "skeleton/main.jsx"}},
		{name: "434_skeleton_head_tsx_both", args: []string{"-F", "-T", "--head", "3", "skeleton/main.tsx"}},
	})
}
