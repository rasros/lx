package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
)

var update = flag.Bool("update", false, "update .golden files")

type goldenTestCase struct {
	name  string
	args  []string
	stdin string
}

// setupMockConfig installs an empty global lx ignore file so real user config
// does not interfere with tests.
func setupMockConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lx"), 0755)
	os.WriteFile(filepath.Join(dir, "lx", "ignore"), []byte(""), 0644)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func writeFile(t *testing.T, dir, path, content string, perm os.FileMode) {
	t.Helper()
	fp := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(fp), err)
	}
	if err := os.WriteFile(fp, []byte(content), perm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func makeSymlink(dir, target, name string) {
	fp := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(fp), 0755)
	_ = os.Symlink(filepath.Join(dir, target), fp)
}

func makeSymlinkRaw(dir, target, name string) {
	fp := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(fp), 0755)
	_ = os.Symlink(target, fp)
}

func buildSymlinksDir(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "links/safe_target/recursion.txt", "I am safe", 0644)
	makeSymlink(dir, "main.go", "links/link_to_main.go")
	makeSymlink(dir, "pkg", "links/link_to_pkg")
	makeSymlinkRaw(dir, "does_not_exist", "links/broken_link")
	makeSymlink(dir, "links/safe_target", "links/loop")
	writeFile(t, dir, "links/cycle_a/visible.txt", "a", 0644)
	writeFile(t, dir, "links/cycle_b/visible.txt", "b", 0644)
	makeSymlinkRaw(dir, "../cycle_b", "links/cycle_a/to_b")
	makeSymlinkRaw(dir, "../cycle_a", "links/cycle_b/to_a")
}

func buildLargeFile(t *testing.T, dir, path string) {
	t.Helper()
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		sb.WriteString("Line ")
		sb.WriteString(strings.Repeat("x", 10))
		sb.WriteString("\n")
	}
	writeFile(t, dir, path, sb.String(), 0644)
}

func runTestGolden(t *testing.T, workDir string, cases []goldenTestCase, extraScrub ...string) {
	t.Helper()
	pkgDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(pkgDir) })
	canonicalWorkDir, _ := filepath.EvalSymlinks(workDir)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	scrub := append([]string{workDir, canonicalWorkDir}, extraScrub...)
	runGoldenTests(t, cases, pkgDir, scrub...)
}

func setupWalkFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "doc/notes.txt", "some notes", 0644)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)

	return dir
}

func setupFormattingFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)

	return dir
}

func setupSectionsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "doc/notes.txt", "some notes", 0644)

	return dir
}

func setupFilteringFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, ".hidden", "i am hidden", 0644)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	writeFile(t, dir, "ignore_test/foo.go", "package foo", 0644)
	writeFile(t, dir, "ignore_test/bar.go", "package bar", 0644)
	writeFile(t, dir, "ignore_test/.gitignore", "bar.go", 0644)

	return dir
}

func setupSlicingFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	buildLargeFile(t, dir, "src/large.txt")

	return dir
}

func setupSymlinksFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	buildSymlinksDir(t, dir)

	return dir
}

func setupErrorsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "spaces/file with spaces.txt", "content with spaces", 0644)

	writeFile(t, dir, "secret/locked.txt", "TOP SECRET", 0600)
	secretDir := filepath.Join(dir, "secret", "locked_dir")
	os.MkdirAll(secretDir, 0755)
	os.WriteFile(filepath.Join(secretDir, "file.txt"), []byte("nested"), 0644)
	os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0000)
	os.Chmod(secretDir, 0000)
	t.Cleanup(func() {
		os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0644)
		os.Chmod(secretDir, 0755)
	})

	return dir
}

func setupStatsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)

	return dir
}

func setupDetectionFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	writeFile(t, dir, "bin/empty.txt", "", 0644)
	writeFile(t, dir, "langs/main.rs", "fn main() {}", 0644)
	writeFile(t, dir, "langs/Dockerfile", "FROM scratch", 0644)
	writeFile(t, dir, "langs/script_no_ext", "#!/bin/bash\necho hi", 0755)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "assets/logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\x0D", 0644)

	return dir
}

func setupConfigFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, ".gitignore", "bin/\n*.tmp\n", 0644)
	writeFile(t, dir, ".hidden", "i am hidden", 0644)
	writeFile(t, dir, "configs/follow.yaml", "follow_symlinks: true\n", 0644)
	writeFile(t, dir, "configs/hidden.yaml", "show_hidden: true\n", 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)
	writeFile(t, dir, "direct_ignore_test/.gitignore", "*.ignored\n", 0644)
	writeFile(t, dir, "direct_ignore_test/test.ignored", "should not appear", 0644)
	writeFile(t, dir, "direct_ignore_test/test.kept", "should appear", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/ignore_me.tmp", "ignore", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/keep_me.go", "package level2", 0644)
	buildSymlinksDir(t, dir)

	return dir
}

func setupComplexFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, "configs/follow.yaml", "follow_symlinks: true\n", 0644)
	writeFile(t, dir, "configs/hidden.yaml", "show_hidden: true\n", 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)
	writeFile(t, dir, "spaces/file with spaces.txt", "content with spaces", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/ignore_me.tmp", "ignore", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/keep_me.go", "package level2", 0644)

	ignoreContent := "*\n!/src/**\n!/migrations/**\n!/assets/**\n!/data/**/*.data.xlsx\n!/data/**/index.json\n!langgraph.json\n!pyproject.toml\n!uv.lock\n"
	writeFile(t, dir, "ignore_exception_test/.gitignore", ignoreContent, 0644)
	writeFile(t, dir, "ignore_exception_test/src/main.go", "package main", 0644)
	writeFile(t, dir, "ignore_exception_test/migrations/001_init.sql", "SELECT 1;", 0644)
	writeFile(t, dir, "ignore_exception_test/assets/logo.png", "image_data", 0644)
	writeFile(t, dir, "ignore_exception_test/data/nested/deep/my.data.xlsx", "excel_data", 0644)
	writeFile(t, dir, "ignore_exception_test/data/index.json", "{}", 0644)
	writeFile(t, dir, "ignore_exception_test/langgraph.json", "{}", 0644)
	writeFile(t, dir, "ignore_exception_test/pyproject.toml", "[tool]", 0644)
	writeFile(t, dir, "ignore_exception_test/uv.lock", "lock_data", 0644)
	writeFile(t, dir, "ignore_exception_test/should_ignore.txt", "ignore me", 0644)
	writeFile(t, dir, "ignore_exception_test/data/secret.csv", "1,2,3", 0644)
	writeFile(t, dir, "ignore_exception_test/data/nested/deep/ignore.xlsx", "ignore", 0644)
	writeFile(t, dir, "ignore_exception_test/other_dir/file.go", "package other", 0644)

	return dir
}

func setupArchiveFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	createTestZip(filepath.Join(dir, "archive.zip"), [][2]string{
		{"hello.txt", "Hello from archive!\n"},
		{"nested/world.go", "package nested\n"},
		{".hidden_in_zip", "hidden inside zip\n"},
	})
	createTestTarGz(filepath.Join(dir, "archive.tar.gz"), [][2]string{
		{"hello.txt", "Hello from tar!\n"},
		{"nested/world.go", "package nested\n"},
	})
	writeFile(t, dir, "configs/expand.yaml", "expand_archives: true\n", 0644)

	return dir
}

func setupDocumentsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	fixtures := []string{
		"sample.pdf", "sample.docx", "sample.xlsx",
		"sample.pptx",
		"sample.odt", "sample.ods", "sample.odp",
	}
	for _, name := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", "documents", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	return dir
}

func setupSkeletonFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "skeleton/main.go", `package demo

type Person struct {
	Name string
	age  int
}

type Worker interface {
	Work()
	rest()
}

func NewPerson(name string) Person {
	return Person{Name: name}
}

func (p Person) Greet() string {
	return p.Name
}

func helper() {}
`, 0644)

	writeFile(t, dir, "skeleton/main.py", `class Animal:
    species = "unknown"
    _tag = "internal"

    def speak(self):
        return "..."

    def _private(self):
        return "x"


def top_level():
    return 1

def _secret():
    return 0
`, 0644)

	writeFile(t, dir, "skeleton/main.c", `typedef struct Point {
    int x;
    int y;
} Point;

int add(int a, int b) {
    return a + b;
}

void greet(const char *name);
`, 0644)

	writeFile(t, dir, "skeleton/main.cpp", `class Widget {
public:
    int value;
    void set(int v) {
        value = v;
    }
private:
    int secret;
    void hide();
};

int FreeFn(int v) {
    return v;
}
`, 0644)

	writeFile(t, dir, "skeleton/Main.java", `public class Calculator {
    private int value;
    public static final int MAX = 100;

    public Calculator(int initial) {
        this.value = initial;
    }

    public int add(int x) {
        return value + x;
    }

    private int secret() {
        return 42;
    }
}
`, 0644)

	writeFile(t, dir, "skeleton/main.rs", `pub struct User {
    pub name: String,
    email: String,
}

pub enum Role {
    Admin,
    User,
}

pub trait Greet {
    fn greet(&self) -> String;
}

impl User {
    pub fn new(name: String) -> Self {
        User { name, email: String::new() }
    }

    pub fn display(&self) -> String {
        self.name.clone()
    }

    fn helper(&self) {}
}

pub fn create(name: String) -> User {
    User::new(name)
}

fn private_fn() {}
`, 0644)

	writeFile(t, dir, "skeleton/main.ts", `export interface Shape {
    area(): number;
    name: string;
}

export class Point {
    public x: number;
    public y: number;
    private label: string;

    constructor(x: number, y: number) {
        this.x = x;
        this.y = y;
        this.label = "";
    }

    public scale(factor: number): Point {
        return new Point(this.x * factor, this.y * factor);
    }

    private helper(): void {}
}

export function topLevel(x: number): number {
    return x + 1;
}
`, 0644)

	writeFile(t, dir, "skeleton/main.kt", `data class Point(val x: Double, val y: Double)

interface Shape {
    fun area(): Double
    fun perimeter(): Double
}

class Calculator {
    val max: Int = 100
    private val secret: Int = 42

    fun add(x: Int): Int {
        return x + max
    }

    private fun helper(): Int = 42
}

fun topLevel(x: Int): Int {
    return x + 1
}
`, 0644)

	writeFile(t, dir, "skeleton/main.rb", `class Animal
  attr_reader :name
  MAX = 100

  def initialize(name)
    @name = name
  end

  def speak
    "..."
  end

  private

  def helper
    42
  end
end

def standalone(x)
  x + 1
end
`, 0644)

	writeFile(t, dir, "skeleton/main.cs", `public interface IShape {
    double Area();
    double Perimeter();
}

public class Point {
    public double X { get; set; }
    public double Y { get; set; }
    private string label;

    public Point(double x, double y) {
        X = x;
        Y = y;
    }

    public Point Scale(double factor) {
        return new Point(X * factor, Y * factor);
    }

    private void Helper() {}
}
`, 0644)

	writeFile(t, dir, "skeleton/main.swift", `public protocol Shape {
    func area() -> Double
    func perimeter() -> Double
}

public struct Point {
    public var x: Double
    public var y: Double
    private var label: String

    public init(x: Double, y: Double) {
        self.x = x
        self.y = y
        self.label = ""
    }

    public func scale(factor: Double) -> Point {
        return Point(x: x * factor, y: y * factor, label: label)
    }

    private func helper() -> Int { 42 }
}

public func topLevel(x: Double) -> Double {
    return x + 1
}
`, 0644)

	writeFile(t, dir, "skeleton/main.scala", `trait Shape {
    def area(): Double
    def perimeter(): Double
}

case class Point(x: Double, y: Double) {
    val label: String = ""
    private val secret: Int = 42

    def scale(factor: Double): Point = Point(x * factor, y * factor)

    private def helper(): Unit = ()
}

object Calculator {
    val Max: Int = 100

    def add(x: Int): Int = x + Max
}

def topLevel(x: Int): Int = x + 1
`, 0644)

	writeFile(t, dir, "skeleton/main.php", `<?php
interface Shape {
    public function area(): float;
    public function perimeter(): float;
}

class Point {
    public float $x;
    public float $y;
    private string $label;

    public function __construct(float $x, float $y) {
        $this->x = $x;
        $this->y = $y;
    }

    public function scale(float $factor): Point {
        return new Point($this->x * $factor, $this->y * $factor);
    }

    private function helper(): void {}
}

function topLevel(int $x): int {
    return $x + 1;
}
`, 0644)

	writeFile(t, dir, "skeleton/main.js", `export class Greeter {
    message = "hi";

    greet(name) {
        return this.message + " " + name;
    }
}

export function topLevel(x) {
    return x + 1;
}
`, 0644)

	writeFile(t, dir, "skeleton/main.jsx", `import React from "react";

export class Card extends React.Component {
    title = "default";

    render() {
        return <div>{this.title}</div>;
    }
}

export function makeCard(name) {
    return <Card title={name} />;
}
`, 0644)

	writeFile(t, dir, "skeleton/main.tsx", `import React from "react";

export interface Props {
    title: string;
}

export class Panel extends React.Component<Props> {
    public value: number;
    private secret: string;

    constructor(props: Props) {
        super(props);
        this.value = 1;
        this.secret = "";
    }

    public render(): JSX.Element {
        return <div>{this.props.title}</div>;
    }
}

export function buildPanel(title: string): JSX.Element {
    return <Panel title={title} />;
}
`, 0644)

	return dir
}

func setupRelativePathsFixture(t *testing.T) (workDir, srcDir string) {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, ".hidden", "i am hidden", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "doc/notes.txt", "some notes", 0644)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)
	buildLargeFile(t, dir, "src/large.txt")
	buildSymlinksDir(t, dir)

	writeFile(t, dir, "secret/locked.txt", "TOP SECRET", 0600)
	os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0000)
	t.Cleanup(func() {
		os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0644)
	})

	return dir, filepath.Join(dir, "src")
}

func runGoldenTests(t *testing.T, cases []goldenTestCase, pkgDir string, scrubPaths ...string) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outR, outW, _ := os.Pipe()
			errR, errW, _ := os.Pipe()
			inR, inW, _ := os.Pipe()

			origOut := os.Stdout
			origErr := os.Stderr
			origIn := os.Stdin

			defer func() {
				os.Stdout = origOut
				os.Stderr = origErr
				os.Stdin = origIn
			}()

			os.Stdout = outW
			os.Stderr = errW
			os.Stdin = inR

			go func() {
				defer inW.Close()
				if tc.stdin != "" {
					io.WriteString(inW, tc.stdin)
				}
			}()

			// Ensure stable output for golden files
			runArgs := append([]string{}, tc.args...)
			hasStatsControl := false
			for _, a := range runArgs {
				if a == "--stats" || a == "--no-stats" || a == "-q" || a == "--quiet" {
					hasStatsControl = true
				}
			}
			if !hasStatsControl {
				runArgs = append(runArgs, "--no-stats")
			}

			errChan := make(chan error, 1)
			go func() {
				defer outW.Close()
				defer errW.Close()
				errChan <- cli.Run(context.Background(), runArgs)
			}()

			var stdoutBuf, stderrBuf bytes.Buffer
			_, _ = io.Copy(&stdoutBuf, outR)
			_, _ = io.Copy(&stderrBuf, errR)

			if err := <-errChan; err != nil {
				stderrBuf.WriteString("\nCLI Error: " + err.Error() + "\n")
			}

			fullOutput := normalizeOutput(stdoutBuf.String(), stderrBuf.String(), scrubPaths...)

			goldenPath := filepath.Join(pkgDir, "testdata", "golden", tc.name+".golden")
			if *update {
				os.MkdirAll(filepath.Dir(goldenPath), 0755)
				os.WriteFile(goldenPath, []byte(fullOutput), 0644)
			}

			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				if *update {
					return
				}
				t.Fatalf("Golden file missing: %v. Run with -update", err)
			}

			if string(wantBytes) != fullOutput {
				t.Errorf("Mismatch for %s.\nExpected len: %d\nGot len: %d\nCheck testdata/golden/%s.golden",
					tc.name, len(wantBytes), len(fullOutput), tc.name)
				_ = os.WriteFile(goldenPath+".actual", []byte(fullOutput), 0644)
			}
		})
	}
}

func normalizeOutput(stdout, stderr string, roots ...string) string {
	var sb strings.Builder

	clean := func(s string) string {
		for _, r := range roots {
			if r != "" {
				s = strings.ReplaceAll(s, r, "/ROOT")
			}
		}

		s = regexp.MustCompile(`(/?\w+)+/TestGolden\w+/\d+`).ReplaceAllString(s, "/ROOT")

		if runtime.GOOS == "windows" {
			s = strings.ReplaceAll(s, "\\", "/")
		}

		s = regexp.MustCompile(`(?i)(permission denied|access is denied)`).ReplaceAllString(s, "PERMISSION_DENIED")
		s = regexp.MustCompile(`(?i)(read .*: is a directory|The handle is invalid)`).ReplaceAllString(s, "IS_DIRECTORY_ERROR")
		s = regexp.MustCompile(`(?i)(The system cannot find the file specified|no such file or directory)`).ReplaceAllString(s, "FILE_NOT_FOUND")
		s = regexp.MustCompile(`time=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+(?:[+-]\d{2}:\d{2}|Z)`).ReplaceAllString(s, "time=FIXED")
		s = regexp.MustCompile(`msg="Loaded global ignore file" path=.*`).ReplaceAllString(s, `msg="Loaded global ignore file" path=GLOBAL_IGNORE`)

		return s
	}

	sb.WriteString("--- STDOUT ---\n")
	sb.WriteString(clean(stdout))
	if !strings.HasSuffix(stdout, "\n") {
		sb.WriteString("\n")
	}

	sb.WriteString("\n--- STDERR ---\n")
	stderrClean := clean(stderr)
	lines := strings.Split(strings.TrimSpace(stderrClean), "\n")
	sort.Strings(lines)
	if len(lines) > 0 && lines[0] != "" {
		sb.WriteString(strings.Join(lines, "\n"))
		sb.WriteString("\n")
	}

	return sb.String()
}

func createTestTarGz(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestTarGz: " + err.Error())
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, file := range files {
		body := []byte(file[1])
		hdr := &tar.Header{Name: file[0], Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			panic("createTestTarGz: " + err.Error())
		}
		if _, err := tw.Write(body); err != nil {
			panic("createTestTarGz: " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		panic("createTestTarGz: " + err.Error())
	}
	if err := gw.Close(); err != nil {
		panic("createTestTarGz: " + err.Error())
	}
}

func createTestZip(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestZip: " + err.Error())
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for _, file := range files {
		fw, err := w.Create(file[0])
		if err != nil {
			panic("createTestZip: " + err.Error())
		}
		if _, err := fw.Write([]byte(file[1])); err != nil {
			panic("createTestZip: " + err.Error())
		}
	}
}
