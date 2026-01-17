package lx

import "testing"

func TestConfig_Merge(t *testing.T) {
	dst := NewConfig()
	src := &Config{OutputFormat: "xml", IgnoreHidden: false}

	Merge(dst, src)

	if dst.OutputFormat != "xml" {
		t.Errorf("Merge failed for OutputFormat")
	}
	if dst.IgnoreHidden {
		t.Errorf("Merge failed for IgnoreHidden")
	}
}
