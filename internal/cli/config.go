package cli

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasros/lx/pkg/lx"
	"gopkg.in/yaml.v3"
)

type CliConfig struct {
	FileContentTemplate string `yaml:"file_content_template"`
	FileErrorTemplate   string `yaml:"file_error_template"`
	FileBinaryTemplate  string `yaml:"file_binary_template"`
	FileCompactTemplate string `yaml:"file_compact_template"`
	FileHeaderTemplate  string `yaml:"file_header_template"`

	SectionTemplate string `yaml:"section_template"`
	PromptTemplate  string `yaml:"prompt_template"`
	TreeTemplate    string `yaml:"tree_template"`
	MetaTemplate    string `yaml:"meta_template"`

	SectionHeaderTemplate string `yaml:"section_header_template"`
	SectionFooterTemplate string `yaml:"section_footer_template"`

	OutputHeaderTemplate string `yaml:"output_header_template"`
	OutputFooterTemplate string `yaml:"output_footer_template"`
	StatsTemplate        string `yaml:"stats_template"`
	ReportTemplate       string `yaml:"report_template"`

	OutputFormat string `yaml:"output_format"`

	OutputMode string `yaml:"output_mode"`
	ShowStats  string `yaml:"show_stats"`
	Verbosity  string `yaml:"verbosity"`

	PromptsDir       string   `yaml:"prompts_dir"`
	PromptExtensions []string `yaml:"prompt_extensions"`
}

func overrideIfSet[T comparable](dst *T, src T) {
	var zero T
	if src != zero {
		*dst = src
	}
}

func LoadConfigChain(cliPath string) (*lx.Config, *CliConfig, error) {
	lxCfg := lx.NewConfig()

	mergedCli := &CliConfig{
		OutputMode: "stdout",
		ShowStats:  "auto",
		Verbosity:  "warn",
	}

	apply := func(path string, strict bool) error {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) && !strict {
				return nil
			}
			slog.Error("Failed to open config file", "path", path, "error", err)
			return err
		}
		defer f.Close()

		slog.Debug("Loading configuration file", "path", path)

		var loaded CliConfig
		if err := yaml.NewDecoder(f).Decode(&loaded); err != nil {
			slog.Error("Failed to parse config file", "path", path, "error", err)
			return err
		}

		overrideIfSet(&lxCfg.FileContentTemplate, loaded.FileContentTemplate)
		overrideIfSet(&lxCfg.FileErrorTemplate, loaded.FileErrorTemplate)
		overrideIfSet(&lxCfg.FileBinaryTemplate, loaded.FileBinaryTemplate)
		overrideIfSet(&lxCfg.FileCompactTemplate, loaded.FileCompactTemplate)
		overrideIfSet(&lxCfg.FileHeaderTemplate, loaded.FileHeaderTemplate)

		overrideIfSet(&lxCfg.SectionTemplate, loaded.SectionTemplate)
		overrideIfSet(&lxCfg.PromptTemplate, loaded.PromptTemplate)
		overrideIfSet(&lxCfg.TreeTemplate, loaded.TreeTemplate)
		overrideIfSet(&lxCfg.MetaTemplate, loaded.MetaTemplate)

		overrideIfSet(&lxCfg.SectionHeaderTemplate, loaded.SectionHeaderTemplate)
		overrideIfSet(&lxCfg.SectionFooterTemplate, loaded.SectionFooterTemplate)

		overrideIfSet(&lxCfg.OutputHeaderTemplate, loaded.OutputHeaderTemplate)
		overrideIfSet(&lxCfg.OutputFooterTemplate, loaded.OutputFooterTemplate)
		overrideIfSet(&lxCfg.StatsTemplate, loaded.StatsTemplate)
		overrideIfSet(&lxCfg.ReportTemplate, loaded.ReportTemplate)

		overrideIfSet(&lxCfg.OutputFormat, loaded.OutputFormat)

		overrideIfSet(&mergedCli.OutputMode, loaded.OutputMode)
		overrideIfSet(&mergedCli.ShowStats, loaded.ShowStats)
		overrideIfSet(&mergedCli.Verbosity, loaded.Verbosity)
		overrideIfSet(&mergedCli.PromptsDir, loaded.PromptsDir)
		if len(loaded.PromptExtensions) > 0 {
			mergedCli.PromptExtensions = loaded.PromptExtensions
		}

		return nil
	}

	if configDir, err := os.UserConfigDir(); err == nil {
		path := filepath.Join(configDir, "lx", "config.yaml")
		_ = apply(path, false)
	}

	if envPath := os.Getenv("LX_CONFIG"); envPath != "" {
		_ = apply(envPath, false)
	}

	if cliPath != "" {
		if err := apply(cliPath, true); err != nil {
			return nil, nil, err
		}
	}

	return lxCfg, mergedCli, nil
}

// LoadGlobalIgnorePatterns returns a slice of strings representing global ignore rules.
func LoadGlobalIgnorePatterns() []string {
	var lines []string
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	candidates := []string{filepath.Join(configDir, "lx", "ignore")}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" && home != "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	if xdgConfig != "" {
		candidates = append(candidates, filepath.Join(xdgConfig, "git", "ignore"))
	}

	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil {
			slog.Debug("Loaded global ignore file", "path", c)
			raw := strings.Split(string(data), "\n")
			for _, line := range raw {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					lines = append(lines, trimmed)
				}
			}
		}
	}

	return lines
}
