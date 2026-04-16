package templatex

import (
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
)

func TestCompileTemplates_Defaults(t *testing.T) {
	cfg := core.NewConfig()
	cfg.OutputFormat = ""

	engine, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Failed to compile default templates: %v", err)
	}
	if engine.FileContent == nil || engine.OutputFooter == nil {
		t.Error("Compile returned nil for required templates")
	}
}

func TestCompileTemplates_InvalidSyntax(t *testing.T) {
	cfg := core.NewConfig()
	cfg.FileContentTemplate = "{{ .Path | badFunction }}"

	_, err := Compile(cfg)
	if err == nil {
		t.Error("Expected error for invalid template syntax, got nil")
	}
}

func TestCompileTemplates_AllFormats(t *testing.T) {
	for _, format := range []string{"markdown", "xml", "html"} {
		t.Run(format, func(t *testing.T) {
			cfg := core.NewConfig()
			cfg.OutputFormat = format

			engine, err := Compile(cfg)
			if err != nil {
				t.Fatalf("Compile(%q) error: %v", format, err)
			}
			if engine.FileContent == nil || engine.Section == nil || engine.Prompt == nil {
				t.Errorf("Compile(%q) returned nil template", format)
			}
		})
	}
}
