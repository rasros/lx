//go:build unix

package cli

import "syscall"

// checkWritable reports why the process could not write to path, as
// access(2) with W_OK finds.
func checkWritable(path string) error {
	const wOK = 2
	return syscall.Access(path, wOK)
}
