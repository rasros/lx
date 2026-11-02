package cli

import (
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

// LoadConfigChain merges the user's config file, the one $LX_CONFIG names and
// the one cliPath names, each over the ones before it.
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
			log.Error("Failed to open config file", "path", path, "error", err)
			return err
		}
		defer f.Close()

		log.Debug("Loading configuration file", "path", path)

		var loaded CliConfig
		if err := yaml.NewDecoder(f).Decode(&loaded); err != nil {
			log.Error("Failed to parse config file", "path", path, "error", err)
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

	if configDir, err := env.configDir(); err == nil {
		path := filepath.Join(configDir, "lx", "config.yaml")
		_ = apply(path, false)
	}

	if envPath := env.getenv("LX_CONFIG"); envPath != "" {
		_ = apply(envPath, false)
	}

	if cliPath != "" {
		if err := apply(cliPath, true); err != nil {
			return nil, nil, err
		}
	}

	return lxCfg, mergedCli, nil
}
