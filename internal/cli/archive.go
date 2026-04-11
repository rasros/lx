package cli

import "github.com/rasros/lx/pkg/lx"

// newArchiveWalker builds a Walker for archive contents.
// Gitignore processing is disabled (archives are pre-packaged), but hidden-file
// filtering is applied based on the config and whether the path is forced.
func newArchiveWalker(cfg *lx.Config, isForced bool) *lx.Walker {
	var overrideRules []string
	if cfg.IgnoreHidden && !isForced {
		overrideRules = append(overrideRules, ".*")
	}
	w := lx.NewWalker(nil, overrideRules)
	w.IgnoreEnabled = false // don't look for .gitignore files inside archives
	return w
}
