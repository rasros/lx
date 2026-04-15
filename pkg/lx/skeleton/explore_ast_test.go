package skeleton

import (
	"fmt"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"testing"
)

func explore(t *testing.T, lang *gotreesitter.Language, code string, langName string) {
	t.Helper()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse([]byte(code))
	if err != nil {
		t.Logf("%s: parse error: %v\n", langName, err)
		return
	}
	root := tree.RootNode()
	t.Logf("=== %s ===\n", langName)
	printNode(t, root, lang, []byte(code), 0, 3)
}

func printNode(t *testing.T, n *gotreesitter.Node, lang *gotreesitter.Language, src []byte, depth, maxDepth int) {
	if depth > maxDepth {
		return
	}
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	text := ""
	if n.IsNamed() && n.ChildCount() == 0 {
		t.Logf("%s[%s]%s", indent, n.Type(lang), fmt.Sprintf(" = %q", n.Text(src)))
		return
	}
	t.Logf("%s[%s]%s", indent, n.Type(lang), text)
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		printNode(t, child, lang, src, depth+1, maxDepth)
	}
}

func TestExploreAST(t *testing.T) {
	explore(t, grammars.BashLanguage(), `
function greet() {
  echo "hello"
}
greet_loud() {
  echo "HELLO"
}
`, "bash")

	explore(t, grammars.PowershellLanguage(), `
function Get-Greeting {
    param($name)
    Write-Host "Hello $name"
}
`, "powershell")

	explore(t, grammars.DartLanguage(), `
class Animal {
  String name;
  Animal(this.name);
  void speak() {}
}
String greet(String name) {
  return "Hello $name";
}
`, "dart")

	explore(t, grammars.LuaLanguage(), `
function greet(name)
  return "Hello " .. name
end
local function helper()
end
`, "lua")

	explore(t, grammars.JuliaLanguage(), `
struct Point
  x::Float64
  y::Float64
end
function greet(name)
  println("Hello " * name)
end
`, "julia")

	explore(t, grammars.ZigLanguage(), `
pub fn add(a: i32, b: i32) i32 {
    return a + b;
}
const Point = struct {
    x: f64,
    y: f64,
};
`, "zig")

	explore(t, grammars.HaskellLanguage(), `
data Color = Red | Green | Blue
newtype Name = Name String
greet :: String -> String
greet name = "Hello " ++ name
`, "haskell")

	explore(t, grammars.GroovyLanguage(), `
class Animal {
    String name
    def speak() {
        println("Hello")
    }
}
def topLevel() {
    println("Top")
}
`, "groovy")

	explore(t, grammars.PerlLanguage(), `
sub greet {
    my $name = shift;
    return "Hello $name";
}
`, "perl")

	explore(t, grammars.OcamlLanguage(), `
type color = Red | Green | Blue
type point = { x: float; y: float }
let greet name = "Hello " ^ name
let add x y = x + y
`, "ocaml")
}

func TestExploreAST2(t *testing.T) {
	explore(t, grammars.ObjcLanguage(), `
@interface Animal : NSObject
@property NSString *name;
- (void)speak;
@end
@implementation Animal
- (void)speak {
    NSLog(@"Hello");
}
@end
void topLevel(int x) {
    return;
}
`, "objc")

	explore(t, grammars.AdaLanguage(), `
package MyPkg is
   type Color is (Red, Green, Blue);
   procedure Greet(Name : String);
   function Add(X, Y : Integer) return Integer;
end MyPkg;
package body MyPkg is
   procedure Greet(Name : String) is
   begin
      null;
   end Greet;
   function Add(X, Y : Integer) return Integer is
   begin
      return X + Y;
   end Add;
end MyPkg;
`, "ada")

	explore(t, grammars.PascalLanguage(), `
type
  TAnimal = class
    Name: string;
    procedure Speak;
    function GetName: string;
  end;
procedure TAnimal.Speak;
begin
  WriteLn('Hello');
end;
function TAnimal.GetName: string;
begin
  Result := Name;
end;
procedure TopLevel(X: Integer);
begin
end;
`, "pascal")

	explore(t, grammars.CommonlispLanguage(), `
(defun greet (name)
  (format t "Hello ~a" name))
(defclass animal ()
  ((name :accessor name :initarg :name)))
(defstruct point
  (x 0.0 :type float)
  (y 0.0 :type float))
`, "commonlisp")
}

func TestExploreAST3(t *testing.T) {
	// Deeper Zig exploration
	explore2 := func(t *testing.T, lang *gotreesitter.Language, code string, name string) {
		t.Helper()
		parser := gotreesitter.NewParser(lang)
		tree, _ := parser.Parse([]byte(code))
		root := tree.RootNode()
		t.Logf("=== %s (deep) ===", name)
		printNode(t, root, lang, []byte(code), 0, 5)
	}
	explore2(t, grammars.ZigLanguage(), `
pub fn add(a: i32, b: i32) i32 {
    return a + b;
}
fn private(x: i32) i32 {
    return x;
}
const Point = struct {
    x: f64,
    y: f64,
};
`, "zig")

	explore2(t, grammars.HaskellLanguage(), `
data Color = Red | Green | Blue
greet :: String -> String
greet name = "Hello " ++ name
add :: Int -> Int -> Int
add x y = x + y
`, "haskell")

	explore2(t, grammars.PowershellLanguage(), `
function Get-Greeting {
    param($name)
    Write-Host "Hello $name"
}
function Set-Name {
    param($name)
}
`, "powershell")

	explore2(t, grammars.DartLanguage(), `
class Animal {
  String name = "dog";
  Animal(this.name);
  void speak() {}
  String get label => name;
}
String greet(String name) {
  return "Hello";
}
`, "dart")

	explore2(t, grammars.ObjcLanguage(), `
@interface Animal : NSObject {
  NSString *_name;
}
@property NSString *name;
- (void)speak;
+ (Animal *)create;
@end
void topLevel(int x) {}
`, "objc")
}

func TestExploreAST4(t *testing.T) {
	// Check named fields
	explore2 := func(t *testing.T, lang *gotreesitter.Language, code string, name string) {
		t.Helper()
		parser := gotreesitter.NewParser(lang)
		tree, _ := parser.Parse([]byte(code))
		root := tree.RootNode()
		t.Logf("=== %s (fields) ===", name)
		printNodeWithFields(t, root, lang, []byte(code), 0, 5)
	}
	explore2(t, grammars.OcamlLanguage(), `
let greet name = "Hello " ^ name
let complex x y =
  let t = x + y in
  t * 2
`, "ocaml-let-binding")

	explore2(t, grammars.PascalLanguage(), `
type
  TAnimal = class
    Name: string;
    procedure Speak;
  end;
procedure TAnimal.Speak;
begin
  WriteLn('Hello');
end;
procedure TopLevel(X: Integer);
begin
end;
`, "pascal")

	explore2(t, grammars.DartLanguage(), `
class Animal {
  String name = "dog";
  Animal(this.name);
  void speak() {}
}
`, "dart-class")

	explore2(t, grammars.LuaLanguage(), `
function greet(name)
  return "Hello"
end
local function helper()
end
MyClass = {}
function MyClass:method()
  return true
end
`, "lua-functions")
}

func printNodeWithFields(t *testing.T, n *gotreesitter.Node, lang *gotreesitter.Language, src []byte, depth, maxDepth int) {
	if depth > maxDepth {
		return
	}
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	text := ""
	if n.IsNamed() && n.ChildCount() == 0 {
		t.Logf("%s[%s]%s", indent, n.Type(lang), fmt.Sprintf(" = %q", n.Text(src)))
		return
	}
	t.Logf("%s[%s]%s (start=%d)", indent, n.Type(lang), text, n.StartPoint().Row)
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		field := n.FieldNameForChild(i, lang)
		if field != "" {
			t.Logf("%s  .%s:", indent, field)
		}
		printNodeWithFields(t, child, lang, src, depth+1, maxDepth)
	}
}
