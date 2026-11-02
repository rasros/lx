//go:build !unix

package cli

import "io/fs"

// soleOwnedLink reports whether renaming a new file over the one info
// describes keeps everything about it but its content. Without a way to see
// its links and owner here, the file is written in place.
func soleOwnedLink(fs.FileInfo) bool { return false }
