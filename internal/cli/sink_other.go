//go:build !unix

package cli

// checkWritable has no portable access(2) to ask here; opening the file at
// commit reports what it finds.
func checkWritable(string) error { return nil }
