//go:build windows

package vault

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// replaceFile uses the Windows replace-existing primitive so a failed write
// never deletes the last good vault.
func replaceFile(source, destination string) error {
	return windows.MoveFileEx(
		windows.StringToUTF16Ptr(source),
		windows.StringToUTF16Ptr(destination),
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

// renameExtractedFile publishes a verified file without replacing a target.
// Prefer a rooted hard link. Some Windows filesystems or security policies
// reject hard links, and os.Root.Rename currently returns ERROR_INVALID_HANDLE
// for this operation, so fall back to MoveFileEx without REPLACE_EXISTING.
func renameExtractedFile(root *os.Root, source, destination string) error {
	if err := root.Link(source, destination); err == nil {
		return nil
	}
	return windows.MoveFileEx(
		windows.StringToUTF16Ptr(filepath.Join(root.Name(), source)),
		windows.StringToUTF16Ptr(filepath.Join(root.Name(), destination)),
		windows.MOVEFILE_WRITE_THROUGH,
	)
}
