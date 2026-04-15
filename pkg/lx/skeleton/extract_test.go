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
