package lx

import (
	"testing"
)

func intPtr(i int) *int { return &i }

func TestOptionsEffective_NoN_UsesHeadTail(t *testing.T) {
	opts := Options{
		Head:  intPtr(2),
		Tail:  intPtr(3),
		NBoth: nil,
	}

	r := opts.ToRunnerConfig()

	if r.Head != 2 || r.Tail != 3 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (2,3)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NOnly(t *testing.T) {
	opts := Options{
		Head:  nil,
		Tail:  nil,
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 3 || r.Tail != 2 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (3,2)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NWithHeadOverride(t *testing.T) {
	opts := Options{
		Head:  intPtr(2),
		Tail:  nil,
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 2 || r.Tail != 3 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (2,3)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NWithTailOverride(t *testing.T) {
	opts := Options{
		Head:  nil,
		Tail:  intPtr(7),
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 0 || r.Tail != 5 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (0,5)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NWithBothOverrides(t *testing.T) {
	opts := Options{
		Head:  intPtr(2),
		Tail:  intPtr(7),
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 2 || r.Tail != 3 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (2,3)", r.Head, r.Tail)
	}
}

func TestCompileTemplates_CustomContent(t *testing.T) {
	cfg := &Config{
		Template: `
MY CUSTOM HEADER
{{ .Content }}
`,
	}

	engine, err := CompileTemplates(cfg)
	if err != nil {
		t.Fatalf("CompileTemplates() error: %v", err)
	}

	if engine.Main == nil {
		t.Fatal("Expected Template to be compiled, got nil")
	}
}

func TestApplyOptions(t *testing.T) {
	cfg := &Config{
		OutputFormat: "markdown",
	}

	opts := Options{
		OutputFormat: "xml",
	}

	ApplyOptions(cfg, opts)

	if cfg.OutputFormat != "xml" {
		t.Errorf("ApplyOptions failed to override format. Got %s, want xml", cfg.OutputFormat)
	}
}
