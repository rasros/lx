package templatex

import (
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
)

func TestCompileTemplates_InvalidSyntax(t *testing.T) {
	cfg := core.NewConfig()
	cfg.FileContentTemplate = "{{ .Path | badFunction }}"

	_, err := Compile(cfg)
	if err == nil {
		t.Error("Expected error for invalid template syntax, got nil")
	}
}
