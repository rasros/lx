package skeleton

import "testing"

// memberCase is one language's type declarations with the skeleton each flag
// combination keeps: -u (functions) keeps method signatures inside the types
// that hold them, -Y (types) keeps types and fields, both keep everything.
type memberCase struct {
	lang, src        string
	functions, types string
	both             string
}

var memberCases = []memberCase{
	{lang: "java", src: `class Shape {
    public int sides;
    private int secret;
    public int area() { return 0; }
    private void hidden() {}
    static class Inner {
        public int depth;
    }
    interface Visitor {
        void visit(Shape s);
    }
}
`,
		functions: `class Shape {
...
    public int area() {
...
    interface Visitor {
        void visit(Shape s);
    }
}
`,
		types: `class Shape {
    public int sides;
...
    static class Inner {
        public int depth;
    }
    interface Visitor {
...
    }
}
`,
		both: `class Shape {
    public int sides;
...
    public int area() {
...
    static class Inner {
        public int depth;
    }
    interface Visitor {
        void visit(Shape s);
    }
}
`,
	},
	{lang: "csharp", src: `public class Shape {
    public int Sides { get; set; }
    private int secret;
    public Shape(int sides) { Sides = sides; }
    private void Hidden() {}
    public struct Inner {
        public int Depth;
    }
}
`,
		functions: `public class Shape {
...
    public Shape(int sides) {
...
}
`,
		types: `public class Shape {
    public int Sides { get; set; }
...
    public struct Inner {
        public int Depth;
    }
}
`,
		both: `public class Shape {
    public int Sides { get; set; }
...
    public Shape(int sides) {
...
    public struct Inner {
        public int Depth;
    }
}
`,
	},
	{lang: "scala", src: `class Shape {
  val sides: Int = 3
  private val secret = 1
  def area(): Double = 0.0
  private def hidden(): Unit = ()
  object Inner {
    val depth = 1
  }
}
`,
		functions: `class Shape {
...
  def area(): Double = 
...
}
`,
		types: `class Shape {
  val sides: Int = 3
...
  object Inner {
    val depth = 1
  }
}
`,
		both: `class Shape {
  val sides: Int = 3
...
  def area(): Double = 
...
  object Inner {
    val depth = 1
  }
}
`,
	},
	{lang: "kotlin", src: `class Shape {
    val sides: Int = 3
    private val secret = 1
    fun area(): Double {
        return 0.0
    }
    private fun hidden() {}
    class Inner {
        val depth = 1
    }
}
`,
		functions: `class Shape {
...
    fun area(): Double {
...
}
`,
		types: `class Shape {
    val sides: Int = 3
...
    class Inner {
        val depth = 1
    }
}
`,
		both: `class Shape {
    val sides: Int = 3
...
    fun area(): Double {
...
    class Inner {
        val depth = 1
    }
}
`,
	},
	{lang: "swift", src: `public protocol Shape {
    var sides: Int { get }
    func area() -> Double
}

public class Square {
    public var side: Double = 1
    private var secret = 1
    public func area() -> Double {
        return side * side
    }
    private func hidden() {}
}
`,
		functions: `public protocol Shape {
...
    func area() -> Double
}
...
public class Square {
...
    public func area() -> Double {
...
}
`,
		types: `public protocol Shape {
    var sides: Int { get }
...
}
...
public class Square {
    public var side: Double = 1
...
}
`,
		both: `public protocol Shape {
    var sides: Int { get }
    func area() -> Double
}
...
public class Square {
    public var side: Double = 1
...
    public func area() -> Double {
...
}
`,
	},
	{lang: "php", src: `<?php
interface Shape {
    public function area(): float;
}

class Square {
    const SIDES = 4;
    public float $side;
    private int $secret;
    public function area(): float {
        return $this->side ** 2;
    }
    private function hidden(): void {}
}
`,
		functions: `interface Shape {
    public function area(): float;
}
...
class Square {
...
    public function area(): float {
...
}
`,
		types: `interface Shape {
...
}
...
class Square {
    const SIDES = 4;
    public float $side;
...
}
`,
		both: `interface Shape {
    public function area(): float;
}
...
class Square {
    const SIDES = 4;
    public float $side;
...
    public function area(): float {
...
}
`,
	},
	{lang: "dart", src: `abstract class Shape {
  final int sides;
  int _secret;
  @override
  double area();
  Shape(this.sides);
  String describe() {
    return 'shape';
  }
  void _hidden() {}
}
`,
		functions: `abstract class Shape {
...
  @override
  double area();
  Shape(this.sides);
  String describe() {
...
}
`,
		types: `abstract class Shape {
  final int sides;
...
}
`,
		both: `abstract class Shape {
  final int sides;
...
  @override
  double area();
  Shape(this.sides);
  String describe() {
...
}
`,
	},
	{lang: "groovy", src: `class Shape {
    int sides
    private int secret
    def area() {
        return 0
    }
    private def hidden() {
    }
}
`,
		functions: `class Shape {
...
    def area() {
...
}
`,
		types: `class Shape {
    int sides
...
}
`,
		both: `class Shape {
    int sides
...
    def area() {
...
}
`,
	},
	{lang: "typescript", src: `export interface Shape {
  sides: number;
  area(): number;
}

export class Square {
  side: number = 1;
  private secret = 1;
  @memo()
  area(): number {
    return this.side ** 2;
  }
  private hidden(): void {}
}
`,
		functions: `export interface Shape {
...
  area(): number;
}
...
export class Square {
...
  @memo()
  area(): number {
...
}
`,
		types: `export interface Shape {
  sides: number;
...
}
...
export class Square {
  side: number = 1;
...
}
`,
		both: `export interface Shape {
  sides: number;
  area(): number;
}
...
export class Square {
  side: number = 1;
...
  @memo()
  area(): number {
...
}
`,
	},
	{lang: "javascript", src: `class Square {
  side = 1;
  area() {
    return this.side ** 2;
  }
}
`,
		functions: `class Square {
...
  area() {
...
}
`,
		types: `class Square {
  side = 1;
...
}
`,
		both: `class Square {
  side = 1;
  area() {
...
}
`,
	},
	{lang: "rust", src: `pub struct Point {
    pub x: f64,
    y: f64,
}

pub enum Color {
    Red,
}

pub trait Area {
    fn area(&self) -> f64;
}

impl Point {
    pub fn new() -> Self {
        Point { x: 0.0, y: 0.0 }
    }
    fn hidden(&self) {}
}
`,
		functions: `pub trait Area {
    fn area(&self) -> f64;
}
...
impl Point {
    pub fn new() -> Self {
...
}
`,
		types: `pub struct Point {
    pub x: f64,
...
}
...
pub enum Color {
    Red,
}
...
pub trait Area {
...
}
...
impl Point {
...
}
`,
		both: `pub struct Point {
    pub x: f64,
...
}
...
pub enum Color {
    Red,
}
...
pub trait Area {
    fn area(&self) -> f64;
}
...
impl Point {
    pub fn new() -> Self {
...
}
`,
	},
	{lang: "cpp", src: `class Widget {
    int implicitly_private;
public:
    int value;
    Widget();
    void set(int v) {
        value = v;
    }
    int get() const;
private:
    int secret;
    void hide();
};
`,
		functions: `class Widget {
...
    Widget();
    void set(int v) {
...
    int get() const;
...
};
`,
		types: `class Widget {
...
    int value;
...
};
`,
		both: `class Widget {
...
    int value;
    Widget();
    void set(int v) {
...
    int get() const;
...
};
`,
	},
}

func TestMembers(t *testing.T) {
	for _, tc := range memberCases {
		for _, mode := range []struct {
			name               string
			functions, structs bool
			want               string
		}{
			{"functions", true, false, tc.functions},
			{"types", false, true, tc.types},
			{"both", true, true, tc.both},
		} {
			t.Run(tc.lang+"/"+mode.name, func(t *testing.T) {
				got := string(Extract(tc.lang, []byte(tc.src), mode.functions, mode.structs))
				if got != mode.want {
					t.Errorf("got:\n%s\nwant:\n%s", got, mode.want)
				}
			})
		}
	}
}

func TestJava_FunctionsOnly(t *testing.T) {
	want := `public class Calculator {
...
    public Calculator(int initial) {
...
    public int add(int x) {
...
    public abstract void reset();
}
`
	if got := string(Extract("java", []byte(javaSrc), true, false)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCSharp_FunctionsOnly(t *testing.T) {
	want := `public interface IShape {
    double Area();
    double Perimeter();
}
...
public class Point {
...
    public Point(double x, double y) {
...
    public Point Scale(double factor) {
...
}
`
	if got := string(Extract("csharp", []byte(csharpSrc), true, false)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFunctionsOnly_LeavesOutTypesWithoutMethods(t *testing.T) {
	for _, tc := range []struct{ lang, src string }{
		{"java", "// Doc.\nclass Point {\n    public int x;\n}\n"},
		{"go", "package p\n\n// Doc.\ntype Point struct {\n\tX int\n}\n"},
		{"python", "# Doc.\nclass Point:\n    x = 1\n"},
		{"ruby", "# Doc.\nclass Point\n  attr_reader :x\nend\n"},
		{"rust", "/// Doc.\npub struct Point {\n    pub x: i32,\n}\n"},
		{"c", "/* Doc. */\ntypedef struct P {\n    int x;\n} P;\n"},
	} {
		if got := Extract(tc.lang, []byte(tc.src), true, false); len(got) != 0 {
			t.Errorf("%s: want nothing, got:\n%s", tc.lang, got)
		}
	}
}

func TestFunctionsOnly_CustomEmittersKeepMethodsInTypes(t *testing.T) {
	for _, tc := range []struct{ lang, src, want string }{
		{"go", `package p

type Shape interface {
	// Area is the area.
	Area() float64
	hidden()
}

type Point struct {
	X int
}
`, `type Shape interface {
	// Area is the area.
	Area() float64
...
}
`},
		{"python", `class Animal:
    """An animal."""
    legs = 4

    def speak(self):
        return "..."
`, `class Animal:
    """An animal."""
...
    def speak(self):
`},
		{"ruby", `class Animal
  MAX = 1
  def speak
    "..."
  end
end
`, `class Animal
...
  def speak
...
end
`},
		{"ocaml", `type t = int

module M = struct
  type u = int
  let area r =
    r * r
end
`, `module M = struct
...
  let area r =
...
end
`},
	} {
		if got := string(Extract(tc.lang, []byte(tc.src), true, false)); got != tc.want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", tc.lang, got, tc.want)
		}
	}
}

func TestDocComments_PreprocessorIsNotDoc(t *testing.T) {
	for _, tc := range []struct{ lang, src, want string }{
		{"c", "#include <stdio.h>\n#define MAX 10\nint f(void);\n", "int f(void);\n"},
		{"cpp", "#include <vector>\n// Doc.\nint f();\n", "// Doc.\nint f();\n"},
		{"objectivec", "#import <Foundation/Foundation.h>\nint f(void) {\n  return 0;\n}\n", "int f(void) {\n"},
		{"csharp", "class A {\n    #region Api\n    public void F() {}\n}\n", "class A {\n...\n    public void F() {\n...\n}\n"},
	} {
		if got := string(Extract(tc.lang, []byte(tc.src), true, true)); got != tc.want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", tc.lang, got, tc.want)
		}
	}
}

func TestDocComments_PerLanguage(t *testing.T) {
	for _, tc := range []struct{ lang, src, want string }{
		// Rust attributes are siblings of their item and stay with it.
		{"rust", "#[inline]\npub fn f() {}\n", "#[inline]\npub fn f() {\n"},
		// PHP has # comments.
		{"php", "<?php\n# Doc.\nfunction f() {}\n", "# Doc.\nfunction f() {\n"},
		// A Python line starting with "//" is not a comment.
		{"python", "x = (1\n// 2)\ndef f():\n    pass\n", "def f():\n"},
	} {
		if got := string(Extract(tc.lang, []byte(tc.src), true, true)); got != tc.want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", tc.lang, got, tc.want)
		}
	}
}
