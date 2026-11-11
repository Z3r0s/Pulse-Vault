//go:build !windows

package vault

import "os"

// replaceFile is atomic on Unix when source and destination share a directory.
func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}

// renameExtractedFile publishes through the open root and preserves the
// platform's same-root path protections.
func renameExtractedFile(root *os.Root, source, destination string) error {
	return root.Rename(source, destination)
}
