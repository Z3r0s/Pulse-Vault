//go:build !windows

package vault

import "os"

// replaceFile is atomic on Unix when source and destination share a directory.
func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
