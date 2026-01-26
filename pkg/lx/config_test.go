package lx

import "testing"

func TestConfig_Merge(t *testing.T) {
	dst := NewConfig()
	dst.OutputFormat = "markdown"
	dst.IgnoreHidden = true

	src := &Config{
		OutputFormat: "xml",
		IgnoreHidden: false,
	}

	Merge(dst, src)

	if dst.OutputFormat != "xml" {
		t.Errorf("Merge failed for OutputFormat")
	}
	if dst.IgnoreHidden {
		t.Errorf("Merge failed for IgnoreHidden")
	}
}

func TestCompileTemplates_Defaults(t *testing.T) {
	cfg := NewConfig()
	cfg.OutputFormat = ""

	engine, err := CompileTemplates(cfg)
	if err != nil {
		t.Fatalf("Failed to compile default templates: %v", err)
	}

	if engine.FileContent == nil || engine.OutputFooter == nil {
		t.Error("CompileTemplates returned nil for required templates")
	}
}

func TestCompileTemplates_InvalidSyntax(t *testing.T) {
	cfg := NewConfig()
	cfg.FileContentTemplate = "{{ .Path | badFunction }}"

	_, err := CompileTemplates(cfg)
	if err == nil {
		t.Error("Expected error for invalid template syntax, got nil")
	}
}
