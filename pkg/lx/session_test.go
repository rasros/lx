package lx

import (
	"io"
	"log/slog"
	"testing"
)

func TestNewSession_Defaults(t *testing.T) {
	cfg := &Config{}
	s, err := NewSession(cfg, nil)
	if err != nil {
		t.Fatalf("NewSession error: %v", err)
	}
	if s.Logger == nil {
		t.Error("NewSession should provide a default logger if nil provided")
	}
	if s.Engine == nil {
		t.Error("NewSession should compile templates")
	}
}

func TestNewSession_InvalidTemplate(t *testing.T) {
	cfg := &Config{
		Template: "{{ .BadSyntax }",
	}
	_, err := NewSession(cfg, nil)
	if err == nil {
		t.Error("NewSession should fail with invalid template")
	}
}

func TestSession_CalculateGlobalContext(t *testing.T) {
	cfg := &Config{}
	s, _ := NewSession(cfg, nil)

	files := []InputFile{
		{Size: 100},
		{Size: 200},
	}

	meta := map[string]string{"foo": "bar"}

	ctx := s.CalculateGlobalContext(files, 5, "/tmp", meta)

	if ctx.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", ctx.TotalFiles)
	}
	if ctx.TotalSize != 300 {
		t.Errorf("TotalSize = %d, want 300", ctx.TotalSize)
	}
	// EstimateTokens is size / 4
	if ctx.TokenEstimate != 75 {
		t.Errorf("TokenEstimate = %d, want 75", ctx.TokenEstimate)
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
	cfg := &Config{Verbosity: "debug"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, _ := NewSession(cfg, logger)

	w := s.NewWalker()
	if w.Config.Verbosity != "debug" {
		t.Error("NewWalker should inherit config")
	}
	if w.Logger != logger {
		t.Error("NewWalker should inherit logger")
	}

	r := s.NewRunner(RunnerConfig{}, GlobalContext{})
	if r.Logger != logger {
		t.Error("NewRunner should inherit logger")
	}
}
