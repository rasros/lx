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

func TestCompileTemplates_AllFormats(t *testing.T) {
	for _, format := range []string{"markdown", "xml", "html"} {
		t.Run(format, func(t *testing.T) {
			cfg := NewConfig()
			cfg.OutputFormat = format

			engine, err := CompileTemplates(cfg)
			if err != nil {
				t.Fatalf("CompileTemplates(%q) error: %v", format, err)
			}
			if engine.FileContent == nil || engine.Section == nil || engine.Prompt == nil {
				t.Errorf("CompileTemplates(%q) returned nil template", format)
			}
		})
	}
}

func TestConfig_Merge_IgnoreFlags(t *testing.T) {
	t.Run("IgnoreFileSymlinks turns on", func(t *testing.T) {
		dst := NewConfig() // default: IgnoreFileSymlinks=false
		Merge(dst, &Config{IgnoreFileSymlinks: true})
		if !dst.IgnoreFileSymlinks {
			t.Error("expected IgnoreFileSymlinks=true after merge")
		}
	})

	t.Run("IgnoreFileSymlinks stays false", func(t *testing.T) {
		dst := NewConfig()
		Merge(dst, &Config{}) // zero value: IgnoreFileSymlinks=false
		if dst.IgnoreFileSymlinks {
			t.Error("expected IgnoreFileSymlinks to remain false")
		}
	})

	t.Run("IgnoreDirSymlinks turns off", func(t *testing.T) {
		dst := NewConfig() // default: IgnoreDirSymlinks=true
		Merge(dst, &Config{IgnoreDirSymlinks: false})
		if dst.IgnoreDirSymlinks {
			t.Error("expected IgnoreDirSymlinks=false after merge")
		}
	})
}
