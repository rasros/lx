package lx

import (
	"testing"
)

func TestNewSession_Defaults(t *testing.T) {
	cfg := &Config{}
	s, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession error: %v", err)
	}
	if s.Engine == nil {
		t.Error("NewSession should compile templates")
	}
}

func TestNewSession_InvalidTemplate(t *testing.T) {
	cfg := &Config{
		Template: "{{ .BadSyntax }",
	}
	_, err := NewSession(cfg)
	if err == nil {
		t.Error("NewSession should fail with invalid template")
	}
}

func TestSession_CalculateGlobalContext(t *testing.T) {
	files := []InputFile{
		{Size: 100},
		{Size: 200},
	}

	meta := map[string]string{"foo": "bar"}

	// CreateGlobalContext is now a package-level helper
	ctx := CreateGlobalContext(files, 5, "/tmp", meta)

	if ctx.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", ctx.TotalFiles)
	}
	if ctx.TotalSize != 300 {
		t.Errorf("TotalSize = %d, want 300", ctx.TotalSize)
	}
	if ctx.TotalSections != 5 {
		t.Errorf("TotalSections = %d, want 5", ctx.TotalSections)
	}
	if ctx.WorkDir != "/tmp" {
		t.Errorf("WorkDir = %q, want /tmp", ctx.WorkDir)
	}
	if ctx.Metadata["foo"] != "bar" {
		t.Errorf("Metadata missing")
	}
}

func TestSession_Factories(t *testing.T) {
	cfg := &Config{}
	s, _ := NewSession(cfg)

	r := s.NewRunner(RunnerConfig{}, GlobalContext{})
	if r == nil {
		t.Error("NewRunner should return a valid runner")
	}
	if r.Engine != s.Engine {
		t.Error("Runner should inherit engine from session")
	}
}
