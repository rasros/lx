package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

// The keys are listed by hand on purpose: they restate the documented schema
// independently of the struct tags, so a mistyped tag leaves its field empty
// here rather than silently dropping a user's setting.
const everyConfigKey = `
file_content_template: "x"
file_error_template: "x"
file_binary_template: "x"
file_compact_template: "x"
file_header_template: "x"
section_template: "x"
prompt_template: "x"
tree_template: "x"
meta_template: "x"
section_header_template: "x"
section_footer_template: "x"
output_header_template: "x"
output_footer_template: "x"
stats_template: "x"
output_format: "xml"
output_mode: "copy"
show_stats: "always"
verbosity: "debug"
prompts_dir: "/tmp/prompts"
prompt_extensions: [".md"]
`

func loadFrom(t *testing.T, yaml string) (*lx.Config, *CliConfig) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("LX_CONFIG", "")

	lxCfg, cliCfg, err := LoadConfigChain(path)
	if err != nil {
		t.Fatalf("LoadConfigChain failed: %v", err)
	}
	return lxCfg, cliCfg
}

func TestLoadConfigChainAcceptsEveryKey(t *testing.T) {
	lxCfg, cliCfg := loadFrom(t, everyConfigKey)

	v := reflect.ValueOf(lxCfg).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).String() == "" {
			t.Errorf("%s stayed empty; check its yaml tag", v.Type().Field(i).Name)
		}
	}

	if cliCfg.OutputMode != "copy" || cliCfg.ShowStats != "always" ||
		cliCfg.Verbosity != "debug" || cliCfg.PromptsDir != "/tmp/prompts" ||
		len(cliCfg.PromptExtensions) != 1 {
		t.Errorf("cli-only keys not applied: %+v", cliCfg)
	}
}

func TestLoadConfigChainKeepsDefaultsForAbsentKeys(t *testing.T) {
	lxCfg, cliCfg := loadFrom(t, "verbosity: info\n")

	if cliCfg.Verbosity != "info" {
		t.Errorf("Verbosity = %q, want info", cliCfg.Verbosity)
	}
	if cliCfg.OutputMode != "stdout" {
		t.Errorf("OutputMode = %q, want the stdout default", cliCfg.OutputMode)
	}
	if lxCfg.OutputFormat != "markdown" {
		t.Errorf("OutputFormat = %q, want the markdown default", lxCfg.OutputFormat)
	}
	if lxCfg.FileContentTemplate != "" {
		t.Errorf("FileContentTemplate = %q, want empty so the format default applies", lxCfg.FileContentTemplate)
	}
}
