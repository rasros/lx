package lx

import (
	"testing"
)

func intPtr(i int) *int { return &i }

func TestOptionsEffective_NoN_UsesHeadTail(t *testing.T) {
	opts := Options{
		Head:  intPtr(2),
		Tail:  intPtr(3),
		NBoth: nil,
	}

	r := opts.ToRunnerConfig()

	if r.Head != 2 || r.Tail != 3 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (2,3)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NOnly(t *testing.T) {
	opts := Options{
		Head:  nil,
		Tail:  nil,
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 3 || r.Tail != 2 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (3,2)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NWithHeadOverride(t *testing.T) {
	opts := Options{
		Head:  intPtr(2),
		Tail:  nil,
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 2 || r.Tail != 3 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (2,3)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NWithTailOverride(t *testing.T) {
	opts := Options{
		Head:  nil,
		Tail:  intPtr(7),
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 0 || r.Tail != 5 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (0,5)", r.Head, r.Tail)
	}
}

func TestOptionsEffective_NWithBothOverrides(t *testing.T) {
	opts := Options{
		Head:  intPtr(2),
		Tail:  intPtr(7),
		NBoth: intPtr(5),
	}

	r := opts.ToRunnerConfig()

	if r.Head != 2 || r.Tail != 3 {
		t.Fatalf("ToRunnerConfig() Head/Tail = (%d,%d), want (2,3)", r.Head, r.Tail)
	}
}

func TestCompileTemplates_CustomContent(t *testing.T) {
	cfg := &Config{
		Template: `
MY CUSTOM HEADER
{{ .Content }}
`,
	}

	engine, err := CompileTemplates(cfg)
	if err != nil {
		t.Fatalf("CompileTemplates() error: %v", err)
	}

	if engine.Main == nil {
		t.Fatal("Expected Template to be compiled, got nil")
	}
}

func TestApplyOptions(t *testing.T) {
	cfg := &Config{
		OutputFormat: "markdown",
	}

	opts := Options{
		OutputFormat: "xml",
	}

	ApplyOptions(cfg, opts)

	if cfg.OutputFormat != "xml" {
		t.Errorf("ApplyOptions failed to override format. Got %s, want xml", cfg.OutputFormat)
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name string
		dst  Config
		src  Config
		want Config
	}{
		{
			name: "strings override if non-empty",
			dst:  Config{Template: "old", OutputFormat: "json"},
			src:  Config{Template: "new", OutputFormat: ""},
			want: Config{Template: "new", OutputFormat: "json"},
		},
		{
			name: "bools override if true",
			dst:  Config{FollowSymlinks: false, ShowHidden: false},
			src:  Config{FollowSymlinks: true, ShowHidden: true},
			want: Config{FollowSymlinks: true, ShowHidden: true},
		},
		{
			name: "bools do not override if false",
			dst:  Config{FollowSymlinks: true},
			src:  Config{FollowSymlinks: false},
			want: Config{FollowSymlinks: true},
		},
		{
			name: "ignore pointer overrides",
			dst:  Config{Ignore: nil},
			src:  Config{Ignore: boolPtr(false)},
			want: Config{Ignore: boolPtr(false)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Merge(&tt.dst, &tt.src)

			// Simple field checks
			if tt.dst.Template != tt.want.Template {
				t.Errorf("Template = %q, want %q", tt.dst.Template, tt.want.Template)
			}
			if tt.dst.OutputFormat != tt.want.OutputFormat {
				t.Errorf("OutputFormat = %q, want %q", tt.dst.OutputFormat, tt.want.OutputFormat)
			}
			if tt.dst.FollowSymlinks != tt.want.FollowSymlinks {
				t.Errorf("FollowSymlinks = %v, want %v", tt.dst.FollowSymlinks, tt.want.FollowSymlinks)
			}
			// Pointer check
			if tt.want.Ignore != nil {
				if tt.dst.Ignore == nil || *tt.dst.Ignore != *tt.want.Ignore {
					t.Errorf("Ignore = %v, want %v", tt.dst.Ignore, tt.want.Ignore)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
