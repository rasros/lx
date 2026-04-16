package main

import "testing"

func TestGoldenSkeleton(t *testing.T) {
	dir := setupSkeletonFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		// 300-309: Go
		{name: "300_skeleton_go_functions", args: []string{"-u", "skeleton/main.go"}},
		{name: "301_skeleton_go_structs", args: []string{"-Y", "skeleton/main.go"}},
		{name: "302_skeleton_go_both", args: []string{"-u", "-Y", "skeleton/main.go"}},

		// 310-319: Python
		{name: "310_skeleton_python_functions", args: []string{"-u", "skeleton/main.py"}},
		{name: "311_skeleton_python_structs", args: []string{"-Y", "skeleton/main.py"}},
		{name: "312_skeleton_python_both", args: []string{"-u", "-Y", "skeleton/main.py"}},

		// 320-329: C-family
		{name: "320_skeleton_c_functions", args: []string{"-u", "skeleton/main.c"}},
		{name: "321_skeleton_c_structs", args: []string{"-Y", "skeleton/main.c"}},
		{name: "322_skeleton_c_both", args: []string{"-u", "-Y", "skeleton/main.c"}},
		{name: "323_skeleton_cpp_both", args: []string{"-u", "-Y", "skeleton/main.cpp"}},

		// 330-339: Java
		{name: "330_skeleton_java_structs", args: []string{"-Y", "skeleton/Main.java"}},
		{name: "331_skeleton_java_both", args: []string{"-u", "-Y", "skeleton/Main.java"}},
		{name: "332_skeleton_java_functions_only", args: []string{"-u", "skeleton/Main.java"}},

		// 340-359: Rust + JS/TS family
		{name: "340_skeleton_rust_both", args: []string{"-u", "-Y", "skeleton/main.rs"}},
		{name: "350_skeleton_typescript_both", args: []string{"-u", "-Y", "skeleton/main.ts"}},
		{name: "351_skeleton_javascript_both", args: []string{"-u", "-Y", "skeleton/main.js"}},
		{name: "352_skeleton_jsx_both", args: []string{"-u", "-Y", "skeleton/main.jsx"}},
		{name: "353_skeleton_tsx_both", args: []string{"-u", "-Y", "skeleton/main.tsx"}},

		// 360-369: Kotlin / Ruby
		{name: "360_skeleton_kotlin_both", args: []string{"-u", "-Y", "skeleton/main.kt"}},
		{name: "361_skeleton_ruby_both", args: []string{"-u", "-Y", "skeleton/main.rb"}},

		// 370-379: C#
		{name: "370_skeleton_csharp_structs", args: []string{"-Y", "skeleton/main.cs"}},
		{name: "371_skeleton_csharp_both", args: []string{"-u", "-Y", "skeleton/main.cs"}},
		{name: "372_skeleton_csharp_functions_only", args: []string{"-u", "skeleton/main.cs"}},

		// 380-399: Swift / Scala / PHP + interleaved toggles
		{name: "380_skeleton_swift_structs", args: []string{"-Y", "skeleton/main.swift"}},
		{name: "381_skeleton_swift_both", args: []string{"-u", "-Y", "skeleton/main.swift"}},
		{name: "390_skeleton_scala_structs", args: []string{"-Y", "skeleton/main.scala"}},
		{name: "391_skeleton_scala_both", args: []string{"-u", "-Y", "skeleton/main.scala"}},
		{name: "392_skeleton_php_structs", args: []string{"-Y", "skeleton/main.php"}},
		{name: "393_skeleton_php_both", args: []string{"-u", "-Y", "skeleton/main.php"}},

		{name: "394_skeleton_interleaved_reset", args: []string{"-u", "-Y", "skeleton/main.py", "--reset-skeleton", "skeleton/main.py"}},
		{name: "395_skeleton_toggle_off", args: []string{"-u", "-Y", "skeleton/main.go", "--reset-skeleton", "skeleton/main.go"}},

		// 500-509: Dart
		{name: "500_skeleton_dart_functions", args: []string{"-u", "skeleton/main.dart"}},
		{name: "501_skeleton_dart_structs", args: []string{"-Y", "skeleton/main.dart"}},
		{name: "502_skeleton_dart_both", args: []string{"-u", "-Y", "skeleton/main.dart"}},

		// 510-519: Lua
		{name: "510_skeleton_lua_functions", args: []string{"-u", "skeleton/main.lua"}},

		// 520-529: Bash
		{name: "520_skeleton_bash_functions", args: []string{"-u", "skeleton/main.sh"}},

		// 530-539: PowerShell
		{name: "530_skeleton_powershell_functions", args: []string{"-u", "skeleton/main.ps1"}},

		// 540-549: Zig
		{name: "540_skeleton_zig_functions", args: []string{"-u", "skeleton/main.zig"}},

		// 550-559: Haskell
		{name: "550_skeleton_haskell_functions", args: []string{"-u", "skeleton/main.hs"}},
		{name: "551_skeleton_haskell_structs", args: []string{"-Y", "skeleton/main.hs"}},
		{name: "552_skeleton_haskell_both", args: []string{"-u", "-Y", "skeleton/main.hs"}},

		// 560-569: Groovy
		{name: "560_skeleton_groovy_functions", args: []string{"-u", "skeleton/main.groovy"}},
		{name: "561_skeleton_groovy_structs", args: []string{"-Y", "skeleton/main.groovy"}},
		{name: "562_skeleton_groovy_both", args: []string{"-u", "-Y", "skeleton/main.groovy"}},

		// 580-589: Objective-C
		{name: "580_skeleton_objc_functions", args: []string{"-u", "skeleton/main.m"}},
		{name: "581_skeleton_objc_structs", args: []string{"-Y", "skeleton/main.m"}},
		{name: "582_skeleton_objc_both", args: []string{"-u", "-Y", "skeleton/main.m"}},

		// 590-599: OCaml
		{name: "590_skeleton_ocaml_structs", args: []string{"-Y", "skeleton/main.ml"}},
		{name: "591_skeleton_ocaml_both", args: []string{"-u", "-Y", "skeleton/main.ml"}},
	})
}

func TestGoldenSkeletonSlicing(t *testing.T) {
	dir := setupSkeletonFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		// 400-409: Go slicing
		{name: "400_skeleton_lines_go_both", args: []string{"-u", "-Y", "--lines", "3", "skeleton/main.go"}},
		{name: "401_skeleton_head_go_both", args: []string{"-u", "-Y", "--head", "2", "skeleton/main.go"}},
		{name: "402_skeleton_tail_go_both", args: []string{"-u", "-Y", "--tail", "2", "skeleton/main.go"}},
		{name: "403_skeleton_head_tail_go_both", args: []string{"-u", "-Y", "--head", "2", "--tail", "2", "skeleton/main.go"}},
		{name: "404_skeleton_progressive_lines_head_tail", args: []string{"-u", "-Y", "--lines", "3", "skeleton/main.go", "--head", "1", "skeleton/main.go", "--tail", "1", "skeleton/main.go"}},

		// 410-419: Existing language slicing parity
		{name: "410_skeleton_lines_python_both", args: []string{"-u", "-Y", "--lines", "2", "skeleton/main.py"}},
		{name: "411_skeleton_head_cpp_both", args: []string{"-u", "-Y", "--head", "3", "skeleton/main.cpp"}},
		{name: "412_skeleton_tail_java_both", args: []string{"-u", "-Y", "--tail", "2", "skeleton/Main.java"}},

		// 420-429: Interleaved state with slicing
		{name: "420_skeleton_lines_after_reset", args: []string{"-u", "-Y", "--reset-skeleton", "--lines", "2", "skeleton/main.py"}},

		// 430-439: New language slicing
		{name: "430_skeleton_head_swift_both", args: []string{"-u", "-Y", "--head", "2", "skeleton/main.swift"}},
		{name: "431_skeleton_tail_scala_both", args: []string{"-u", "-Y", "--tail", "2", "skeleton/main.scala"}},
		{name: "432_skeleton_lines_php_both", args: []string{"-u", "-Y", "--lines", "3", "skeleton/main.php"}},
		{name: "433_skeleton_lines_jsx_both", args: []string{"-u", "-Y", "--lines", "2", "skeleton/main.jsx"}},
		{name: "434_skeleton_head_tsx_both", args: []string{"-u", "-Y", "--head", "3", "skeleton/main.tsx"}},
	})
}
