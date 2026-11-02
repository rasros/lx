package cli

import "github.com/rasros/lx/pkg/lx"

// newArchiveWalker builds a Walker for archive contents. Exclude patterns
// become override rules relative to the archive root, the same way they are
// relative to a walked directory.
func newArchiveWalker(excludes []string, showHidden bool, onIgnore func(path, reason string)) *lx.Walker {
	return lx.NewWalker(lx.WalkerOptions{
		Override:   excludes,
		SkipHidden: !showHidden,
		OnIgnore:   onIgnore,
	})
}
