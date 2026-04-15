package skeleton

import (
	"strings"
	"testing"
)

var goSrc = `package geo

import "fmt"

type Point struct {
	X float64
	Y float64
	label string
}

type Shape interface {
	Area() float64
	Perimeter() float64
}

func NewPoint(x, y float64) Point {
	return Point{x, y, ""}
}

func (p Point) Scale(factor float64) Point {
	return Point{p.X * factor, p.Y * factor, p.label}
}

func (p Point) String() string {
	return fmt.Sprintf("(%g, %g)", p.X, p.Y)
}

func (p Point) helper() {}
`

func TestGo_Functions(t *testing.T) {
	out := string(Extract("go", []byte(goSrc), true, false))
	if !strings.Contains(out, "func NewPoint(x, y float64) Point {") {
		t.Errorf("expected NewPoint, got:\n%s", out)
	}
	if !strings.Contains(out, "func (p Point) Scale(factor float64) Point {") {
		t.Errorf("expected Scale, got:\n%s", out)
	}
	if !strings.Contains(out, "func (p Point) String() string {") {
		t.Errorf("expected String, got:\n%s", out)
	}
	if strings.Contains(out, "helper") {
		t.Errorf("unexported helper should not appear:\n%s", out)
	}
	if strings.Contains(out, "return Point{") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestGo_Structs(t *testing.T) {
	out := string(Extract("go", []byte(goSrc), false, true))
	if !strings.Contains(out, "type Point struct {") {
		t.Errorf("expected Point struct, got:\n%s", out)
	}
	if !strings.Contains(out, "\tX float64") {
		t.Errorf("expected exported X field, got:\n%s", out)
	}
	if !strings.Contains(out, "\tY float64") {
		t.Errorf("expected exported Y field, got:\n%s", out)
	}
	if strings.Contains(out, "label") {
		t.Errorf("unexported label field should not appear:\n%s", out)
	}
	if !strings.Contains(out, "type Shape interface {") {
		t.Errorf("expected Shape interface, got:\n%s", out)
	}
	if strings.Contains(out, "Area()") {
		t.Errorf("interface methods should not appear in structs-only mode:\n%s", out)
	}
}

func TestGo_Both(t *testing.T) {
	out := string(Extract("go", []byte(goSrc), true, true))
	if !strings.Contains(out, "type Point struct {") {
		t.Errorf("expected Point struct")
	}
	if !strings.Contains(out, "\tX float64") {
		t.Errorf("expected X field")
	}
	if !strings.Contains(out, "func NewPoint(x, y float64) Point {") {
		t.Errorf("expected NewPoint")
	}
	if !strings.Contains(out, "func (p Point) Scale(factor float64) Point {") {
		t.Errorf("expected Scale")
	}
	if !strings.Contains(out, "Area() float64") {
		t.Errorf("expected Shape interface method Area")
	}
	if strings.Contains(out, "helper") {
		t.Errorf("unexported helper should not appear:\n%s", out)
	}
}

var cSrc = `#include <stdio.h>

typedef struct Point {
    int x;
    int y;
} Point;

int add(int a, int b) {
    return a + b;
}

void greet(const char *name);

static int helper(void) {
    int z = 1;
    return z;
}
`

func TestC_Functions(t *testing.T) {
	out := string(Extract("c", []byte(cSrc), true, false))
	if !strings.Contains(out, "int add(int a, int b) {") {
		t.Errorf("expected add() signature, got:\n%s", out)
	}
	if !strings.Contains(out, "void greet(const char *name);") {
		t.Errorf("expected greet() declaration, got:\n%s", out)
	}
	if !strings.Contains(out, "static int helper(void) {") {
		t.Errorf("expected helper() signature, got:\n%s", out)
	}
	if strings.Contains(out, "return a + b") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestC_Structs(t *testing.T) {
	out := string(Extract("c", []byte(cSrc), false, true))
	if !strings.Contains(out, "typedef struct Point {") {
		t.Errorf("expected struct definition, got:\n%s", out)
	}
	if !strings.Contains(out, "int x;") {
		t.Errorf("expected struct field x, got:\n%s", out)
	}
	if !strings.Contains(out, "int y;") {
		t.Errorf("expected struct field y, got:\n%s", out)
	}
	if strings.Contains(out, "return a + b") {
		t.Errorf("function body should not appear:\n%s", out)
	}
	if strings.Contains(out, "int add") {
		t.Errorf("function signatures should not appear in structs-only mode:\n%s", out)
	}
}

func TestC_ControlFlowExcluded(t *testing.T) {
	src := []byte(`void foo(void) {
    if (x > 0) {
        return;
    }
    for (int i = 0; i < 10; i++) {
    }
}
`)
	out := string(Extract("c", src, true, false))
	if !strings.Contains(out, "void foo(void) {") {
		t.Errorf("expected foo() signature, got:\n%s", out)
	}
	if strings.Contains(out, "if (x > 0)") {
		t.Errorf("if statement should not appear:\n%s", out)
	}
	if strings.Contains(out, "for") {
		t.Errorf("for loop should not appear:\n%s", out)
	}
}

var javaSrc = `package com.example;

import java.util.List;

public class Calculator {
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

    public abstract void reset();
}
`

func TestJava_Structs(t *testing.T) {
	out := string(Extract("java", []byte(javaSrc), false, true))
	if !strings.Contains(out, "public class Calculator {") {
		t.Errorf("expected class declaration, got:\n%s", out)
	}
	if !strings.Contains(out, "public static final int MAX = 100;") {
		t.Errorf("expected public field, got:\n%s", out)
	}
	if strings.Contains(out, "private int value") {
		t.Errorf("private field should not appear:\n%s", out)
	}
	if strings.Contains(out, "add(") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
	if strings.Contains(out, "return value + x") {
		t.Errorf("method body should not appear:\n%s", out)
	}
}

func TestJava_Both(t *testing.T) {
	out := string(Extract("java", []byte(javaSrc), true, true))
	if !strings.Contains(out, "public class Calculator {") {
		t.Errorf("expected class declaration")
	}
	if !strings.Contains(out, "public static final int MAX = 100;") {
		t.Errorf("expected public field")
	}
	if !strings.Contains(out, "public Calculator(int initial) {") {
		t.Errorf("expected constructor signature")
	}
	if !strings.Contains(out, "public int add(int x) {") {
		t.Errorf("expected add() signature")
	}
	if !strings.Contains(out, "public abstract void reset();") {
		t.Errorf("expected reset() declaration")
	}
	if strings.Contains(out, "private int value") {
		t.Errorf("private field should not appear:\n%s", out)
	}
	if strings.Contains(out, "private int secret") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if strings.Contains(out, "return value + x") {
		t.Errorf("method body should not appear:\n%s", out)
	}
}

func TestJava_FunctionsOnlyNoClassMembers(t *testing.T) {
	out := string(Extract("java", []byte(javaSrc), true, false))
	if strings.Contains(out, "public int add") {
		t.Errorf("Java methods should not appear with functions flag alone:\n%s", out)
	}
}

var pySrc = `"""Module docstring."""

MAX = 100

class Animal:
    species = "unknown"
    legs: int = 4
    _tag = "internal"

    def __init__(self, name):
        self.name = name

    def speak(self):
        return "..."

    def _helper(self):
        x = 1
        return x


def standalone(x, y):
    result = x + y
    return result

def _private_helper():
    pass


class Dog(Animal):
    def speak(self):
        return "woof"
`

func TestPython_Functions(t *testing.T) {
	out := string(Extract("python", []byte(pySrc), true, false))
	if !strings.Contains(out, "def standalone(x, y):") {
		t.Errorf("expected standalone(), got:\n%s", out)
	}
	if strings.Contains(out, "def __init__") {
		t.Errorf("class method should not appear without structs flag:\n%s", out)
	}
	if strings.Contains(out, "def speak") {
		t.Errorf("class method should not appear without structs flag:\n%s", out)
	}
	if strings.Contains(out, "_private_helper") {
		t.Errorf("private function should not appear:\n%s", out)
	}
}

func TestPython_Structs(t *testing.T) {
	out := string(Extract("python", []byte(pySrc), false, true))
	if !strings.Contains(out, "class Animal:") {
		t.Errorf("expected Animal class, got:\n%s", out)
	}
	if !strings.Contains(out, `    species = "unknown"`) {
		t.Errorf("expected species class var, got:\n%s", out)
	}
	if !strings.Contains(out, "    legs: int = 4") {
		t.Errorf("expected legs annotation, got:\n%s", out)
	}
	if strings.Contains(out, "_tag") {
		t.Errorf("private class var should not appear:\n%s", out)
	}
	if !strings.Contains(out, "class Dog(Animal):") {
		t.Errorf("expected Dog class, got:\n%s", out)
	}
	if strings.Contains(out, "def speak") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
	if strings.Contains(out, "def standalone") {
		t.Errorf("standalone function should not appear in structs-only mode:\n%s", out)
	}
}

func TestPython_Both(t *testing.T) {
	out := string(Extract("python", []byte(pySrc), true, true))
	if !strings.Contains(out, "class Animal:") {
		t.Errorf("expected class")
	}
	if !strings.Contains(out, `    species = "unknown"`) {
		t.Errorf("expected class variable")
	}
	if !strings.Contains(out, "    def __init__(self, name):") {
		t.Errorf("expected __init__")
	}
	if !strings.Contains(out, "    def speak(self):") {
		t.Errorf("expected speak()")
	}
	if strings.Contains(out, "_helper") {
		t.Errorf("private _helper should not appear:\n%s", out)
	}
	if !strings.Contains(out, "def standalone(x, y):") {
		t.Errorf("expected standalone()")
	}
	if !strings.Contains(out, "class Dog(Animal):") {
		t.Errorf("expected Dog class")
	}
	if strings.Contains(out, "self.name = name") {
		t.Errorf("__init__ body should not appear:\n%s", out)
	}
}

func TestPython_DecoratorSkipped(t *testing.T) {
	src := []byte(`class SwitchTenantRequest(BaseModel):
    tenant_id: UUID

@router.put("/logo", status_code=status.HTTP_204_NO_CONTENT)
async def update_logo() -> None:
    pass

@router.get(
    "/logo",
    status_code=status.HTTP_200_OK,
)
async def get_logo() -> None:
    pass
`)
	out := string(Extract("python", src, true, true))
	if !strings.Contains(out, "class SwitchTenantRequest(BaseModel):") {
		t.Errorf("expected class, got:\n%s", out)
	}
	if strings.Contains(out, "@router") {
		t.Errorf("decorator should not appear:\n%s", out)
	}
	if strings.Contains(out, "status_code") {
		t.Errorf("decorator args should not appear:\n%s", out)
	}
	if !strings.Contains(out, "async def update_logo() -> None:") {
		t.Errorf("expected update_logo, got:\n%s", out)
	}
	if !strings.Contains(out, "async def get_logo() -> None:") {
		t.Errorf("expected get_logo, got:\n%s", out)
	}
}

func TestPython_ClassVarMultiLineValue(t *testing.T) {
	src := []byte(`class StubBackend(BaseBackend):
    backend = SomeBackend(
        model_id="some-model", gpu_memory_utilization=0.65, max_new_tokens=512
    )
    name: str = "stub"

def test_something():
    assert result == expected
`)
	out := string(Extract("python", src, false, true))
	if !strings.Contains(out, "class StubBackend(BaseBackend):") {
		t.Errorf("expected class, got:\n%s", out)
	}
	if !strings.Contains(out, "    backend = SomeBackend(") {
		t.Errorf("expected class variable declaration, got:\n%s", out)
	}
	if strings.Contains(out, "model_id=") {
		t.Errorf("constructor args should not appear:\n%s", out)
	}
	if !strings.Contains(out, `    name: str = "stub"`) {
		t.Errorf("expected name field after constructor close, got:\n%s", out)
	}
	if strings.Contains(out, "assert") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestPython_ModuleLevelAfterClass(t *testing.T) {
	src := []byte(`class Example(BaseModel):
    query: str
    label: str

REGISTRY: list[Example] = [
    Example(query="foo", label="bar"),
]
`)
	out := string(Extract("python", src, false, true))
	if !strings.Contains(out, "class Example(BaseModel):") {
		t.Errorf("expected class declaration, got:\n%s", out)
	}
	if !strings.Contains(out, "    query: str") {
		t.Errorf("expected class field, got:\n%s", out)
	}
	if strings.Contains(out, "REGISTRY") {
		t.Errorf("module-level variable should not appear:\n%s", out)
	}
	if strings.Contains(out, `query="foo"`) {
		t.Errorf("list literal contents should not appear:\n%s", out)
	}
}

func TestPython_MultiLineSignature(t *testing.T) {
	src := []byte(`class TestFoo:
    def test_one(
        self, pg_pool: ConnectionPool, tmp_path: Path
    ) -> None:
        app_db = AppDatabase(pg_pool)
        assert app_db is not None

    def test_two(self) -> None:
        pass
`)
	out := string(Extract("python", src, true, true))
	if !strings.Contains(out, "    def test_one(") {
		t.Errorf("expected test_one opening, got:\n%s", out)
	}
	if !strings.Contains(out, "        self, pg_pool: ConnectionPool, tmp_path: Path") {
		t.Errorf("expected signature args, got:\n%s", out)
	}
	if !strings.Contains(out, "    ) -> None:") {
		t.Errorf("expected signature closing, got:\n%s", out)
	}
	if !strings.Contains(out, "    def test_two(self) -> None:") {
		t.Errorf("expected test_two signature, got:\n%s", out)
	}
	if strings.Contains(out, "app_db") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestPython_NestedFunctionExcluded(t *testing.T) {
	src := []byte(`def outer():
    x = 1
    def inner():
        y = 2
    return x
`)
	out := string(Extract("python", src, true, false))
	if !strings.Contains(out, "def outer():") {
		t.Errorf("expected outer(), got:\n%s", out)
	}
	if strings.Contains(out, "def inner():") {
		t.Errorf("nested inner() should not appear:\n%s", out)
	}
}

var rustSrc = `pub struct Point {
    pub x: f64,
    pub y: f64,
    label: String,
}

pub enum Color {
    Red,
    Green,
    Blue,
}

pub trait Shape {
    fn area(&self) -> f64;
    fn perimeter(&self) -> f64;
}

impl Point {
    pub fn new(x: f64, y: f64) -> Self {
        Point { x, y, label: String::new() }
    }

    pub fn scale(&self, factor: f64) -> Point {
        Point { x: self.x * factor, y: self.y * factor, label: self.label.clone() }
    }

    fn helper(&self) {}
}

pub fn top_level(x: i32) -> i32 {
    x + 1
}

fn private_fn() {}
`

func TestRust_Functions(t *testing.T) {
	out := string(Extract("rust", []byte(rustSrc), true, false))
	if !strings.Contains(out, "pub fn top_level(x: i32) -> i32 {") {
		t.Errorf("expected top_level, got:\n%s", out)
	}
	if strings.Contains(out, "private_fn") {
		t.Errorf("private fn should not appear:\n%s", out)
	}
	if strings.Contains(out, "fn helper") {
		t.Errorf("private helper should not appear:\n%s", out)
	}
	if strings.Contains(out, "x + 1") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestRust_Structs(t *testing.T) {
	out := string(Extract("rust", []byte(rustSrc), false, true))
	if !strings.Contains(out, "pub struct Point {") {
		t.Errorf("expected struct Point, got:\n%s", out)
	}
	if !strings.Contains(out, "pub x: f64,") {
		t.Errorf("expected pub field x, got:\n%s", out)
	}
	if !strings.Contains(out, "pub y: f64,") {
		t.Errorf("expected pub field y, got:\n%s", out)
	}
	if strings.Contains(out, "label") {
		t.Errorf("private field label should not appear:\n%s", out)
	}
	if !strings.Contains(out, "pub enum Color {") {
		t.Errorf("expected enum Color, got:\n%s", out)
	}
	if !strings.Contains(out, "Red,") {
		t.Errorf("expected enum variant Red, got:\n%s", out)
	}
	if !strings.Contains(out, "pub trait Shape {") {
		t.Errorf("expected trait Shape, got:\n%s", out)
	}
	if !strings.Contains(out, "impl Point {") {
		t.Errorf("expected impl Point, got:\n%s", out)
	}
}

func TestRust_Both(t *testing.T) {
	out := string(Extract("rust", []byte(rustSrc), true, true))
	if !strings.Contains(out, "pub struct Point {") {
		t.Errorf("expected struct Point")
	}
	if !strings.Contains(out, "fn area(&self) -> f64;") {
		t.Errorf("expected trait method area, got:\n%s", out)
	}
	if !strings.Contains(out, "pub fn new(x: f64, y: f64) -> Self {") {
		t.Errorf("expected impl method new, got:\n%s", out)
	}
	if !strings.Contains(out, "pub fn scale(&self, factor: f64) -> Point {") {
		t.Errorf("expected impl method scale, got:\n%s", out)
	}
	if strings.Contains(out, "fn helper") {
		t.Errorf("private helper should not appear:\n%s", out)
	}
	if !strings.Contains(out, "pub fn top_level(x: i32) -> i32 {") {
		t.Errorf("expected top_level, got:\n%s", out)
	}
	if strings.Contains(out, "x + 1") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

var tsSrc = `export interface Shape {
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

function localFn(): void {}
`

func TestTypeScript_Functions(t *testing.T) {
	out := string(Extract("typescript", []byte(tsSrc), true, false))
	if !strings.Contains(out, "export function topLevel(x: number): number {") {
		t.Errorf("expected exported topLevel, got:\n%s", out)
	}
	if !strings.Contains(out, "function localFn(): void {") {
		t.Errorf("expected non-exported localFn, got:\n%s", out)
	}
	if strings.Contains(out, "return x + 1") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestTypeScript_Structs(t *testing.T) {
	out := string(Extract("typescript", []byte(tsSrc), false, true))
	if !strings.Contains(out, "export interface Shape {") {
		t.Errorf("expected Shape interface, got:\n%s", out)
	}
	if !strings.Contains(out, "area(): number;") {
		t.Errorf("expected interface method area, got:\n%s", out)
	}
	if !strings.Contains(out, "export class Point {") {
		t.Errorf("expected Point class, got:\n%s", out)
	}
	if !strings.Contains(out, "public x: number;") {
		t.Errorf("expected public field x, got:\n%s", out)
	}
	if strings.Contains(out, "private label") {
		t.Errorf("private field should not appear:\n%s", out)
	}
	if strings.Contains(out, "scale(") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
}

func TestTypeScript_Both(t *testing.T) {
	out := string(Extract("typescript", []byte(tsSrc), true, true))
	if !strings.Contains(out, "export interface Shape {") {
		t.Errorf("expected Shape interface")
	}
	if !strings.Contains(out, "export class Point {") {
		t.Errorf("expected Point class")
	}
	if !strings.Contains(out, "public x: number;") {
		t.Errorf("expected public field x")
	}
	if !strings.Contains(out, "constructor(x: number, y: number) {") {
		t.Errorf("expected constructor")
	}
	if !strings.Contains(out, "public scale(factor: number): Point {") {
		t.Errorf("expected scale method")
	}
	if strings.Contains(out, "private helper") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if strings.Contains(out, "this.x = x") {
		t.Errorf("constructor body should not appear:\n%s", out)
	}
}

var kotlinSrc = `data class Point(val x: Double, val y: Double)

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
`

func TestKotlin_Functions(t *testing.T) {
	out := string(Extract("kotlin", []byte(kotlinSrc), true, false))
	if !strings.Contains(out, "fun topLevel(x: Int): Int {") {
		t.Errorf("expected topLevel, got:\n%s", out)
	}
	if strings.Contains(out, "return x + 1") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestKotlin_Structs(t *testing.T) {
	out := string(Extract("kotlin", []byte(kotlinSrc), false, true))
	if !strings.Contains(out, "data class Point(val x: Double, val y: Double)") {
		t.Errorf("expected Point data class, got:\n%s", out)
	}
	if !strings.Contains(out, "interface Shape {") {
		t.Errorf("expected Shape interface, got:\n%s", out)
	}
	if !strings.Contains(out, "class Calculator {") {
		t.Errorf("expected Calculator class, got:\n%s", out)
	}
	if !strings.Contains(out, "val max: Int = 100") {
		t.Errorf("expected public property max, got:\n%s", out)
	}
	if strings.Contains(out, "private val secret") {
		t.Errorf("private property should not appear:\n%s", out)
	}
	if strings.Contains(out, "fun add") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
}

func TestKotlin_Both(t *testing.T) {
	out := string(Extract("kotlin", []byte(kotlinSrc), true, true))
	if !strings.Contains(out, "fun area(): Double") {
		t.Errorf("expected interface method area, got:\n%s", out)
	}
	if !strings.Contains(out, "fun add(x: Int): Int {") {
		t.Errorf("expected add method, got:\n%s", out)
	}
	if strings.Contains(out, "private fun helper") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if strings.Contains(out, "return x + max") {
		t.Errorf("method body should not appear:\n%s", out)
	}
}

var csharpSrc = `public interface IShape {
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
`

func TestCSharp_Structs(t *testing.T) {
	out := string(Extract("csharp", []byte(csharpSrc), false, true))
	if !strings.Contains(out, "public interface IShape {") {
		t.Errorf("expected IShape interface, got:\n%s", out)
	}
	if !strings.Contains(out, "public class Point {") {
		t.Errorf("expected Point class, got:\n%s", out)
	}
	if !strings.Contains(out, "public double X { get; set; }") {
		t.Errorf("expected public property X, got:\n%s", out)
	}
	if strings.Contains(out, "private string label") {
		t.Errorf("private field should not appear:\n%s", out)
	}
	if strings.Contains(out, "Scale(") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
}

func TestCSharp_Both(t *testing.T) {
	out := string(Extract("csharp", []byte(csharpSrc), true, true))
	if !strings.Contains(out, "double Area();") {
		t.Errorf("expected interface method Area, got:\n%s", out)
	}
	if !strings.Contains(out, "public Point(double x, double y) {") {
		t.Errorf("expected constructor, got:\n%s", out)
	}
	if !strings.Contains(out, "public Point Scale(double factor) {") {
		t.Errorf("expected Scale method, got:\n%s", out)
	}
	if strings.Contains(out, "private void Helper") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if strings.Contains(out, "X = x") {
		t.Errorf("constructor body should not appear:\n%s", out)
	}
}

var rubySrc = `class Animal
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
`

func TestRuby_Functions(t *testing.T) {
	out := string(Extract("ruby", []byte(rubySrc), true, false))
	if !strings.Contains(out, "def standalone(x)") {
		t.Errorf("expected standalone, got:\n%s", out)
	}
	if strings.Contains(out, "x + 1") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}

func TestRuby_Structs(t *testing.T) {
	out := string(Extract("ruby", []byte(rubySrc), false, true))
	if !strings.Contains(out, "class Animal") {
		t.Errorf("expected Animal class, got:\n%s", out)
	}
	if !strings.Contains(out, "attr_reader :name") {
		t.Errorf("expected attr_reader, got:\n%s", out)
	}
	if !strings.Contains(out, "MAX = 100") {
		t.Errorf("expected constant MAX, got:\n%s", out)
	}
	if strings.Contains(out, "def standalone") {
		t.Errorf("standalone function should not appear in structs-only mode:\n%s", out)
	}
}

func TestRuby_Both(t *testing.T) {
	out := string(Extract("ruby", []byte(rubySrc), true, true))
	if !strings.Contains(out, "class Animal") {
		t.Errorf("expected Animal class")
	}
	if !strings.Contains(out, "def initialize(name)") {
		t.Errorf("expected initialize method, got:\n%s", out)
	}
	if !strings.Contains(out, "def speak") {
		t.Errorf("expected speak method, got:\n%s", out)
	}
	if strings.Contains(out, "def helper") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if !strings.Contains(out, "def standalone(x)") {
		t.Errorf("expected standalone function, got:\n%s", out)
	}
	if strings.Contains(out, "@name = name") {
		t.Errorf("method body should not appear:\n%s", out)
	}
}

var swiftSrc = `public protocol Shape {
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
`

func TestSwift_Structs(t *testing.T) {
	out := string(Extract("swift", []byte(swiftSrc), false, true))
	if !strings.Contains(out, "public protocol Shape {") {
		t.Errorf("expected Shape protocol, got:\n%s", out)
	}
	if !strings.Contains(out, "public struct Point {") {
		t.Errorf("expected Point struct, got:\n%s", out)
	}
	if !strings.Contains(out, "public var x: Double") {
		t.Errorf("expected public property x, got:\n%s", out)
	}
	if strings.Contains(out, "private var label") {
		t.Errorf("private property should not appear:\n%s", out)
	}
	if strings.Contains(out, "func scale") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
}

func TestSwift_Both(t *testing.T) {
	out := string(Extract("swift", []byte(swiftSrc), true, true))
	if !strings.Contains(out, "public init(x: Double, y: Double) {") {
		t.Errorf("expected init, got:\n%s", out)
	}
	if !strings.Contains(out, "public func scale(factor: Double) -> Point {") {
		t.Errorf("expected scale, got:\n%s", out)
	}
	if strings.Contains(out, "private func helper") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if !strings.Contains(out, "public func topLevel(x: Double) -> Double {") {
		t.Errorf("expected topLevel, got:\n%s", out)
	}
	if strings.Contains(out, "self.x = x") {
		t.Errorf("init body should not appear:\n%s", out)
	}
}

var scalaSrc = `trait Shape {
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
`

func TestScala_Structs(t *testing.T) {
	out := string(Extract("scala", []byte(scalaSrc), false, true))
	if !strings.Contains(out, "trait Shape {") {
		t.Errorf("expected Shape trait, got:\n%s", out)
	}
	if !strings.Contains(out, "case class Point(x: Double, y: Double) {") {
		t.Errorf("expected Point class, got:\n%s", out)
	}
	if !strings.Contains(out, `val label: String = ""`) {
		t.Errorf("expected public val label, got:\n%s", out)
	}
	if strings.Contains(out, "private val secret") {
		t.Errorf("private val should not appear:\n%s", out)
	}
	if !strings.Contains(out, "object Calculator {") {
		t.Errorf("expected Calculator object, got:\n%s", out)
	}
	if strings.Contains(out, "def scale") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
}

func TestScala_Both(t *testing.T) {
	out := string(Extract("scala", []byte(scalaSrc), true, true))
	if !strings.Contains(out, "def area(): Double") {
		t.Errorf("expected trait method area, got:\n%s", out)
	}
	if !strings.Contains(out, "def scale(factor: Double): Point") {
		t.Errorf("expected scale method, got:\n%s", out)
	}
	if strings.Contains(out, "private def helper") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if !strings.Contains(out, "def add(x: Int): Int") {
		t.Errorf("expected add method, got:\n%s", out)
	}
	if !strings.Contains(out, "def topLevel(x: Int): Int") {
		t.Errorf("expected topLevel, got:\n%s", out)
	}
}

var phpSrc = `<?php
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
`

func TestPHP_Structs(t *testing.T) {
	out := string(Extract("php", []byte(phpSrc), false, true))
	if !strings.Contains(out, "interface Shape {") {
		t.Errorf("expected Shape interface, got:\n%s", out)
	}
	if !strings.Contains(out, "class Point {") {
		t.Errorf("expected Point class, got:\n%s", out)
	}
	if !strings.Contains(out, "public float $x;") {
		t.Errorf("expected public property x, got:\n%s", out)
	}
	if strings.Contains(out, "private string $label") {
		t.Errorf("private property should not appear:\n%s", out)
	}
	if strings.Contains(out, "function scale") {
		t.Errorf("methods should not appear in structs-only mode:\n%s", out)
	}
}

func TestPHP_Both(t *testing.T) {
	out := string(Extract("php", []byte(phpSrc), true, true))
	if !strings.Contains(out, "public function area(): float;") {
		t.Errorf("expected interface method area, got:\n%s", out)
	}
	if !strings.Contains(out, "public function __construct(float $x, float $y) {") {
		t.Errorf("expected constructor, got:\n%s", out)
	}
	if !strings.Contains(out, "public function scale(float $factor): Point {") {
		t.Errorf("expected scale method, got:\n%s", out)
	}
	if strings.Contains(out, "private function helper") {
		t.Errorf("private method should not appear:\n%s", out)
	}
	if !strings.Contains(out, "function topLevel(int $x): int {") {
		t.Errorf("expected topLevel, got:\n%s", out)
	}
	if strings.Contains(out, "return $x + 1") {
		t.Errorf("function body should not appear:\n%s", out)
	}
}
