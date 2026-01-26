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
	// File-level Templates
	FileContentTemplate string `yaml:"file_content_template"`
	FileErrorTemplate   string `yaml:"file_error_template"`
	FileBinaryTemplate  string `yaml:"file_binary_template"`
	FileCompactTemplate string `yaml:"file_compact_template"`
	FileHeaderTemplate  string `yaml:"file_header_template"`

	// Item-level Templates
	SectionTemplate string `yaml:"section_template"`
	PromptTemplate  string `yaml:"prompt_template"`

	// Group/Wrapper Templates
	SectionHeaderTemplate string `yaml:"section_header_template"`
	SectionFooterTemplate string `yaml:"section_footer_template"`

	// Global Output Templates
	OutputHeaderTemplate string `yaml:"output_header_template"`
	OutputFooterTemplate string `yaml:"output_footer_template"`
	StatsTemplate        string `yaml:"stats_template"`

	OutputFormat string `yaml:"output_format"`

	FollowSymlinks *bool `yaml:"follow_symlinks"`
	NoFileSymlinks *bool `yaml:"no_file_links"`

	ShowHidden *bool `yaml:"show_hidden"`
	Ignore     *bool `yaml:"ignore"`

	OutputMode string `yaml:"output_mode"`
	ShowStats  string `yaml:"show_stats"`
	Verbosity  string `yaml:"verbosity"`
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

		if loaded.FileContentTemplate != "" {
			lxCfg.FileContentTemplate = loaded.FileContentTemplate
		}
		if loaded.FileErrorTemplate != "" {
			lxCfg.FileErrorTemplate = loaded.FileErrorTemplate
		}
		if loaded.FileBinaryTemplate != "" {
			lxCfg.FileBinaryTemplate = loaded.FileBinaryTemplate
		}
		if loaded.FileCompactTemplate != "" {
			lxCfg.FileCompactTemplate = loaded.FileCompactTemplate
		}
		if loaded.FileHeaderTemplate != "" {
			lxCfg.FileHeaderTemplate = loaded.FileHeaderTemplate
		}

		if loaded.SectionTemplate != "" {
			lxCfg.SectionTemplate = loaded.SectionTemplate
		}
		if loaded.PromptTemplate != "" {
			lxCfg.PromptTemplate = loaded.PromptTemplate
		}

		if loaded.SectionHeaderTemplate != "" {
			lxCfg.SectionHeaderTemplate = loaded.SectionHeaderTemplate
		}
		if loaded.SectionFooterTemplate != "" {
			lxCfg.SectionFooterTemplate = loaded.SectionFooterTemplate
		}

		if loaded.OutputHeaderTemplate != "" {
			lxCfg.OutputHeaderTemplate = loaded.OutputHeaderTemplate
		}
		if loaded.OutputFooterTemplate != "" {
			lxCfg.OutputFooterTemplate = loaded.OutputFooterTemplate
		}
		if loaded.StatsTemplate != "" {
			lxCfg.StatsTemplate = loaded.StatsTemplate
		}

		if loaded.OutputFormat != "" {
			lxCfg.OutputFormat = loaded.OutputFormat
		}

		if loaded.FollowSymlinks != nil {
			lxCfg.IgnoreDirSymlinks = !(*loaded.FollowSymlinks)
		}
		if loaded.NoFileSymlinks != nil {
			lxCfg.IgnoreFileSymlinks = *loaded.NoFileSymlinks
		}
		if loaded.ShowHidden != nil {
			lxCfg.IgnoreHidden = !(*loaded.ShowHidden)
		}
		if loaded.Ignore != nil {
			lxCfg.IgnoreEnabled = *loaded.Ignore
		}

		if loaded.OutputMode != "" {
			mergedCli.OutputMode = loaded.OutputMode
		}
		if loaded.ShowStats != "" {
			mergedCli.ShowStats = loaded.ShowStats
		}
		if loaded.Verbosity != "" {
			mergedCli.Verbosity = loaded.Verbosity
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
