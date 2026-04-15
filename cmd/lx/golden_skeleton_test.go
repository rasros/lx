package main

import "testing"

func TestGoldenSkeleton(t *testing.T) {
	dir := setupSkeletonFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		// 300-309: Go
		{name: "300_skeleton_go_functions", args: []string{"-F", "skeleton/main.go"}},
		{name: "301_skeleton_go_structs", args: []string{"-T", "skeleton/main.go"}},
		{name: "302_skeleton_go_both", args: []string{"-F", "-T", "skeleton/main.go"}},

		// 310-319: Python
		{name: "310_skeleton_python_functions", args: []string{"-F", "skeleton/main.py"}},
		{name: "311_skeleton_python_structs", args: []string{"-T", "skeleton/main.py"}},
		{name: "312_skeleton_python_both", args: []string{"-F", "-T", "skeleton/main.py"}},

		// 320-329: C-family
		{name: "320_skeleton_c_functions", args: []string{"-F", "skeleton/main.c"}},
		{name: "321_skeleton_c_structs", args: []string{"-T", "skeleton/main.c"}},
		{name: "322_skeleton_c_both", args: []string{"-F", "-T", "skeleton/main.c"}},
		{name: "323_skeleton_cpp_both", args: []string{"-F", "-T", "skeleton/main.cpp"}},

		// 330-339: Java
		{name: "330_skeleton_java_structs", args: []string{"-T", "skeleton/Main.java"}},
		{name: "331_skeleton_java_both", args: []string{"-F", "-T", "skeleton/Main.java"}},
		{name: "332_skeleton_java_functions_only", args: []string{"-F", "skeleton/Main.java"}},

		// 340-359: Rust + JS/TS family
		{name: "340_skeleton_rust_both", args: []string{"-F", "-T", "skeleton/main.rs"}},
		{name: "350_skeleton_typescript_both", args: []string{"-F", "-T", "skeleton/main.ts"}},
		{name: "351_skeleton_javascript_both", args: []string{"-F", "-T", "skeleton/main.js"}},
		{name: "352_skeleton_jsx_both", args: []string{"-F", "-T", "skeleton/main.jsx"}},
		{name: "353_skeleton_tsx_both", args: []string{"-F", "-T", "skeleton/main.tsx"}},

		// 360-369: Kotlin / Ruby
		{name: "360_skeleton_kotlin_both", args: []string{"-F", "-T", "skeleton/main.kt"}},
		{name: "361_skeleton_ruby_both", args: []string{"-F", "-T", "skeleton/main.rb"}},

		// 370-379: C#
		{name: "370_skeleton_csharp_structs", args: []string{"-T", "skeleton/main.cs"}},
		{name: "371_skeleton_csharp_both", args: []string{"-F", "-T", "skeleton/main.cs"}},
		{name: "372_skeleton_csharp_functions_only", args: []string{"-F", "skeleton/main.cs"}},

		// 380-399: Swift / Scala / PHP + interleaved toggles
		{name: "380_skeleton_swift_structs", args: []string{"-T", "skeleton/main.swift"}},
		{name: "381_skeleton_swift_both", args: []string{"-F", "-T", "skeleton/main.swift"}},
		{name: "390_skeleton_scala_structs", args: []string{"-T", "skeleton/main.scala"}},
		{name: "391_skeleton_scala_both", args: []string{"-F", "-T", "skeleton/main.scala"}},
		{name: "392_skeleton_php_structs", args: []string{"-T", "skeleton/main.php"}},
		{name: "393_skeleton_php_both", args: []string{"-F", "-T", "skeleton/main.php"}},

		{name: "394_skeleton_interleaved_reset", args: []string{"-F", "-T", "skeleton/main.py", "--reset-skeleton", "skeleton/main.py"}},
		{name: "395_skeleton_toggle_off_functions", args: []string{"-F", "-T", "skeleton/main.go", "--no-functions", "skeleton/main.go"}},
		{name: "396_skeleton_toggle_off_structs", args: []string{"-F", "-T", "skeleton/main.go", "--no-structs", "skeleton/main.go"}},
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
