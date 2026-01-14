package lx

import "testing"

func TestConfig_Merge(t *testing.T) {
	dst := NewConfig()
	src := &Config{OutputFormat: "xml", ShowHidden: true}

	Merge(dst, src)

	if dst.OutputFormat != "xml" {
		t.Errorf("Merge failed for OutputFormat")
	}
	if !dst.ShowHidden {
		t.Errorf("Merge failed for ShowHidden")
	}
}
