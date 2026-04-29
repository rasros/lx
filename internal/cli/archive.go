package cli

import "github.com/rasros/lx/pkg/lx"

// newArchiveWalker builds a Walker for archive contents.
func newArchiveWalker(showHidden bool, isForced bool) *lx.Walker {
	w := lx.NewWalker(nil, nil)
	w.IgnoreEnabled = false
	w.SkipHidden = !showHidden && !isForced
	return w
}
