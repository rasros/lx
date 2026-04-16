package core

import "testing"

func TestConfig_Merge(t *testing.T) {
	dst := NewConfig()
	dst.OutputFormat = "markdown"
	dst.IgnoreHidden = true

	src := &Config{OutputFormat: "xml", IgnoreHidden: false}
	Merge(dst, src)

	if dst.OutputFormat != "xml" {
		t.Errorf("Merge failed for OutputFormat")
	}
	if dst.IgnoreHidden {
		t.Errorf("Merge failed for IgnoreHidden")
	}
}

func TestConfig_Merge_IgnoreFlags(t *testing.T) {
	t.Run("IgnoreFileSymlinks turns on", func(t *testing.T) {
		dst := NewConfig()
		Merge(dst, &Config{IgnoreFileSymlinks: true})
		if !dst.IgnoreFileSymlinks {
			t.Error("expected IgnoreFileSymlinks=true after merge")
		}
	})

	t.Run("IgnoreFileSymlinks stays false", func(t *testing.T) {
		dst := NewConfig()
		Merge(dst, &Config{})
		if dst.IgnoreFileSymlinks {
			t.Error("expected IgnoreFileSymlinks to remain false")
		}
	})

	t.Run("IgnoreDirSymlinks turns off", func(t *testing.T) {
		dst := NewConfig()
		Merge(dst, &Config{IgnoreDirSymlinks: false})
		if dst.IgnoreDirSymlinks {
			t.Error("expected IgnoreDirSymlinks=false after merge")
		}
	})
}
