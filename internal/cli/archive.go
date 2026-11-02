package cli

import "github.com/rasros/lx/pkg/lx"

// newArchiveWalker builds a Walker for archive contents. Exclude patterns
// become override rules relative to the archive root, the same way they are
// relative to a walked directory. A forced archive (-f) bypasses them.
func newArchiveWalker(excludes []string, showHidden bool, isForced bool, onIgnore func(path, reason string)) *lx.Walker {
	var overrideRules []string
	if !isForced {
		overrideRules = excludes
	}
	return lx.NewWalker(lx.WalkerOptions{
		Override:   overrideRules,
		SkipHidden: !showHidden && !isForced,
		OnIgnore:   onIgnore,
	})
}
