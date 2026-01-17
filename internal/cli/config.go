package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
	"github.com/rasros/lx/pkg/lx"
	"gopkg.in/yaml.v3"
)

type CliConfig struct {
	Template              string `yaml:"template"`
	SectionTemplate       string `yaml:"section_template"`
	PromptTemplate        string `yaml:"prompt_template"`
	StatsTemplate         string `yaml:"stats_template"`
	HeaderTemplate        string `yaml:"header_template"`
	FooterTemplate        string `yaml:"footer_template"`
	SectionHeaderTemplate string `yaml:"section_header_template"`
	SectionFooterTemplate string `yaml:"section_footer_template"`
	OutputFormat          string `yaml:"output_format"`

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

		if loaded.Template != "" {
			lxCfg.Template = loaded.Template
		}
		if loaded.SectionTemplate != "" {
			lxCfg.SectionTemplate = loaded.SectionTemplate
		}
		if loaded.PromptTemplate != "" {
			lxCfg.PromptTemplate = loaded.PromptTemplate
		}
		if loaded.StatsTemplate != "" {
			lxCfg.StatsTemplate = loaded.StatsTemplate
		}
		if loaded.HeaderTemplate != "" {
			lxCfg.HeaderTemplate = loaded.HeaderTemplate
		}
		if loaded.FooterTemplate != "" {
			lxCfg.FooterTemplate = loaded.FooterTemplate
		}
		if loaded.SectionHeaderTemplate != "" {
			lxCfg.SectionHeaderTemplate = loaded.SectionHeaderTemplate
		}
		if loaded.SectionFooterTemplate != "" {
			lxCfg.SectionFooterTemplate = loaded.SectionFooterTemplate
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

	lxCfg.GlobalIgnore = loadGlobalIgnores()
	return lxCfg, mergedCli, nil
}

func loadGlobalIgnores() gitignore.IgnoreMatcher {
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
			lines = append(lines, strings.Split(string(data), "\n")...)
		}
	}

	if len(lines) == 0 {
		return nil
	}
	buf := bytes.NewBufferString(strings.Join(lines, "\n"))
	return gitignore.NewGitIgnoreFromReader(".", buf)
}
