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
	// The library config keys are inlined from their single definition in
	// core.Config, so adding a template needs no change here.
	lx.Config `yaml:",inline"`

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

		lx.Merge(lxCfg, &loaded.Config)

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
