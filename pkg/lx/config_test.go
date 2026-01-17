package lx

import "testing"

func TestConfig_Merge(t *testing.T) {
	dst := NewConfig()
	// src: We want to ENABLE hidden files, so IgnoreHidden = false
	src := &Config{OutputFormat: "xml", IgnoreHidden: false}

	Merge(dst, src)

	if dst.OutputFormat != "xml" {
		t.Errorf("Merge failed for OutputFormat")
	}
	// dst should now have IgnoreHidden = false
	if dst.IgnoreHidden {
		t.Errorf("Merge failed for IgnoreHidden")
	}
}
