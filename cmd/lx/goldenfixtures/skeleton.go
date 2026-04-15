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
		"main.lua",
		"main.sh",
		"main.ps1",
		"main.zig",
		"main.hs",
		"main.groovy",
		"main.m",
		"main.ml",
	}
	for _, name := range languageFiles {
		content := readFixtureFile(t, filepath.Join("languages", name))
		writeFile(t, dir, filepath.Join("skeleton", name), string(content), 0644)
	}

	return dir
}
