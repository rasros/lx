package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/rasros/lx/pkg/lx"
	"gopkg.in/yaml.v3"
)

type CliConfig struct {
	// The library config keys are inlined from their single definition in
	// core.Config, so adding a template needs no change here.
	lx.Config `yaml:",inline"`

	OutputMode string `yaml:"output_mode"`
	ShowStats  string `yaml:"show_stats"`
	// StatsTemplate renders the summary of a run, a StatsContext.
	StatsTemplate string `yaml:"stats_template"`
	Verbosity     string `yaml:"verbosity"`

	PromptsDir       string   `yaml:"prompts_dir"`
	PromptExtensions []string `yaml:"prompt_extensions"`
}

func overrideIfSet[T comparable](dst *T, src T) {
	var zero T
	if src != zero {
		*dst = src
	}
}

// LoadConfigChain merges the user's config file, the one $LX_CONFIG names and
// the one cliPath names, each over the ones before it. A missing user or
// $LX_CONFIG file is skipped and one that fails is logged and skipped; a
// cliPath file that fails is the error, which is left to the caller to
// report.
func LoadConfigChain(env Env, log *slog.Logger, cliPath string) (*lx.Config, *CliConfig, error) {
	lxCfg := lx.NewConfig()

	mergedCli := &CliConfig{
		OutputMode: "stdout",
		ShowStats:  "auto",
		Verbosity:  "warn",
	}

	apply := func(path string, strict bool) error {
		f, err := env.open(path)
		if err != nil {
			if os.IsNotExist(err) && !strict {
				return nil
			}
			return fmt.Errorf("config file: %w", err)
		}
		defer f.Close()

		log.Debug("Loading configuration file", "path", path)

		var loaded CliConfig
		if err := yaml.NewDecoder(f).Decode(&loaded); err != nil {
			return fmt.Errorf("parse config file %s: %w", path, err)
		}

		lx.Merge(lxCfg, &loaded.Config)

		overrideIfSet(&mergedCli.OutputMode, loaded.OutputMode)
		overrideIfSet(&mergedCli.ShowStats, loaded.ShowStats)
		overrideIfSet(&mergedCli.StatsTemplate, loaded.StatsTemplate)
		overrideIfSet(&mergedCli.Verbosity, loaded.Verbosity)
		overrideIfSet(&mergedCli.PromptsDir, loaded.PromptsDir)
		if len(loaded.PromptExtensions) > 0 {
			mergedCli.PromptExtensions = loaded.PromptExtensions
		}

		return nil
	}

	if configDir, err := env.configDir(); err == nil {
		if err := apply(filepath.Join(configDir, "lx", "config.yaml"), false); err != nil {
			log.Error("Skipping config file", "error", err)
		}
	}

	if envPath := env.getenv("LX_CONFIG"); envPath != "" {
		if err := apply(envPath, false); err != nil {
			log.Error("Skipping config file", "error", err)
		}
	}

	if cliPath != "" {
		if err := apply(cliPath, true); err != nil {
			return nil, nil, err
		}
	}

	return lxCfg, mergedCli, nil
}
