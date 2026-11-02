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

// LoadGlobalIgnorePatterns returns the global ignore patterns: lx's own,
// which apply everywhere, and git's global excludes, which apply only inside
// a git repository.
func LoadGlobalIgnorePatterns() (lxPatterns, gitPatterns []string) {
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	lxPatterns = readIgnorePatterns(filepath.Join(configDir, "lx", "ignore"))

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" && home != "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	if xdgConfig != "" {
		gitPatterns = readIgnorePatterns(filepath.Join(xdgConfig, "git", "ignore"))
	}
	return lxPatterns, gitPatterns
}

// readIgnorePatterns returns the patterns in the ignore file at p, if any.
func readIgnorePatterns(p string) []string {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	slog.Debug("Loaded global ignore file", "path", p)
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
