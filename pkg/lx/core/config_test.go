package core

import "testing"

func TestConfig_Merge(t *testing.T) {
	dst := NewConfig()
	dst.OutputFormat = "markdown"

	src := &Config{OutputFormat: "xml"}
	Merge(dst, src)

	if dst.OutputFormat != "xml" {
		t.Errorf("Merge failed for OutputFormat")
	}
}
