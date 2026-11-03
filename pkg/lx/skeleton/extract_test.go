package skeleton

import (
	"strings"
	"testing"
)

func TestGo_GroupedTypeDeclNoDuplication(t *testing.T) {
	src := []byte(`package p

type (
	Public struct {
		X int
		y int
	}
	alias int
	Name string
	API interface {
		Call()
		hidden()
	}
)
`)
	out := string(Extract("go", false, src, true, true))
	if strings.Count(out, "type (") != 1 {
		t.Errorf("expected grouped type header once, got:\n%s", out)
	}
	if strings.Count(out, "Public struct {") != 1 {
		t.Errorf("expected Public struct once, got:\n%s", out)
	}
	if !strings.Contains(out, "X int") {
		t.Errorf("expected exported field X, got:\n%s", out)
	}
	if strings.Contains(out, "y int") {
		t.Errorf("unexported field y should not appear:\n%s", out)
	}
	if strings.Contains(out, "alias int") {
		t.Errorf("unexported alias should not appear:\n%s", out)
	}
	if !strings.Contains(out, "Name string") {
		t.Errorf("expected exported alias Name, got:\n%s", out)
	}
	if !strings.Contains(out, "Call()") {
		t.Errorf("expected exported interface method Call, got:\n%s", out)
	}
	if strings.Contains(out, "hidden()") {
		t.Errorf("unexported interface method should not appear:\n%s", out)
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
	out := string(Extract("c", false, src, true, false))
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

func TestC_SingleLineBodyExcluded(t *testing.T) {
	src := []byte(`int add(int a, int b) { return a + b; }
`)
	out := string(Extract("c", false, src, true, false))
	if !strings.Contains(out, "int add(int a, int b)") {
		t.Errorf("expected add() signature, got:\n%s", out)
	}
	if strings.Contains(out, "return a + b") {
		t.Errorf("single-line function body should not appear:\n%s", out)
	}
}

func TestJava_AnnotatedPrivateMethodExcluded(t *testing.T) {
	src := []byte(`class A {
	@Deprecated
	private int secret() { return 1; }
	public int ok() { return 2; }
}
`)
	out := string(Extract("java", false, src, true, true))
	if strings.Contains(out, "secret(") {
		t.Errorf("annotated private method should not appear:\n%s", out)
	}
	if !strings.Contains(out, "ok()") {
		t.Errorf("expected public method ok(), got:\n%s", out)
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
	out := string(Extract("python", false, src, false, true))
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
	out := string(Extract("python", false, src, false, true))
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
	out := string(Extract("python", false, src, true, true))
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
	out := string(Extract("python", false, src, true, false))
	if !strings.Contains(out, "def outer():") {
		t.Errorf("expected outer(), got:\n%s", out)
	}
	if strings.Contains(out, "def inner():") {
		t.Errorf("nested inner() should not appear:\n%s", out)
	}
}
