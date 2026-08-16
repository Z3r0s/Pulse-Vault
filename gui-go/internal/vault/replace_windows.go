//go:build windows

package vault

import (
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
