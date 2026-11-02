//go:build unix

package cli

import (
	"io/fs"
	"os"
	"syscall"
)

// soleOwnedLink reports whether renaming a new file over the one info
// describes keeps everything about it but its content: it has no other links,
// and the run owns it, so the new file gets its owner and group.
func soleOwnedLink(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Nlink == 1 && int(st.Uid) == os.Geteuid() && int(st.Gid) == os.Getegid()
}
