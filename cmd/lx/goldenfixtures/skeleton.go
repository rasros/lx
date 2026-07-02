package goldenfixtures

import (
	"path/filepath"
	"testing"
)

func SetupSkeletonFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	languageFiles := []string{
		"main.go",
		"main.py",
		"fstring.py",
		"commentbody.py",
		"commentbody.rb",
		"commentbody.hs",
		"main.c",
		"main.cpp",
		"Main.java",
		"main.rs",
		"main.ts",
		"main.kt",
		"main.rb",
		"main.cs",
		"main.swift",
		"main.scala",
		"main.php",
		"main.js",
		"main.jsx",
		"main.tsx",
		"main.dart",
		"main.zig",
		"main.hs",
		"main.groovy",
		"main.m",
		"main.ml",
		"deco.py",
		"deco.ts",
		"deco.tsx",
		"Deco.java",
		"deco.kt",
		"deco.rs",
		"deco.cs",
		"deco.scala",
		"deco.swift",
		"deco.dart",
	}
	for _, name := range languageFiles {
		content := readFixtureFile(t, filepath.Join("languages", name))
		writeFile(t, dir, filepath.Join("skeleton", name), string(content), 0644)
	}

	return dir
}
