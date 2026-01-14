package lx

import (
	"os"
	"path/filepath"
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

func TestOptionsEffective_LoadYaml(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "lx.yaml")
	yamlContent := `
template: |
  MY CUSTOM HEADER
  {{ .Content }}
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		ConfigPath: configPath,
	}

	engine, _, err := opts.CompileTemplates()
	if err != nil {
		t.Fatalf("CompileTemplates() error: %v", err)
	}

	if engine.Main == nil {
		t.Fatal("Expected Template to be compiled, got nil")
	}
}

func TestOptionsEffective_MissingConfig(t *testing.T) {
	opts := Options{
		ConfigPath: "/path/to/non/existent/file.yaml",
	}

	_, _, err := opts.CompileTemplates()
	if err == nil {
		t.Fatal("Expected error for missing config file, got nil")
	}
}

func TestOptionsEffective_ConfigMerging(t *testing.T) {
	tmpDir := t.TempDir()

	envCfgPath := filepath.Join(tmpDir, "env_config.yaml")
	envContent := `
template: "ENV_TEMPLATE"
section_template: "ENV_SECTION"
`
	if err := os.WriteFile(envCfgPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	cliCfgPath := filepath.Join(tmpDir, "cli_config.yaml")
	cliContent := `
template: "CLI_TEMPLATE"
`
	if err := os.WriteFile(cliCfgPath, []byte(cliContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LX_CONFIG", envCfgPath)

	opts := Options{
		ConfigPath: cliCfgPath,
	}

	engine, _, err := opts.CompileTemplates()
	if err != nil {
		t.Fatalf("CompileTemplates failed: %v", err)
	}

	if engine.Main == nil {
		t.Fatal("Engine Main is nil")
	}
}
