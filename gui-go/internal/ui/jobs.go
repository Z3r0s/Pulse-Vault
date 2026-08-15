package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// JobState tracks long-running vault work for the UI (pure; no Fyne dependency).
type JobState struct {
	mu      sync.Mutex
	busy    bool
	op      string
	message string
}

// ErrBusy is returned when Begin is called while a job is already running.
var ErrBusy = errors.New("another operation is already in progress")

// Begin marks the UI busy with an operation label and status message.
func (j *JobState) Begin(op, message string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.busy {
		return ErrBusy
	}
	j.busy = true
	j.op = op
	j.message = message
	return nil
}

// Finish clears the busy flag and sets a terminal status message.
func (j *JobState) Finish(message string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.busy = false
	j.op = ""
	j.message = message
}

// Snapshot returns busy, op, and message for UI refresh.
func (j *JobState) Snapshot() (busy bool, op, message string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.busy, j.op, j.message
}

// Busy reports whether a job is running.
func (j *JobState) Busy() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.busy
}

// StatusLine formats a user-facing status string for the current snapshot.
func StatusLine(busy bool, op, message string) string {
	if !busy {
		return message
	}
	if op == "" {
		return "Working… " + message
	}
	return fmt.Sprintf("%s… %s", op, message)
}

// PrepareCreatePath normalizes a file-save path and lists empty placeholders to remove
// before vault.Create (Fyne ShowFileSave creates the path first).
func PrepareCreatePath(path string) (final string, remove []string) {
	if path == "" {
		return "", nil
	}
	remove = []string{path}
	final = path
	if !strings.HasSuffix(strings.ToLower(path), ".pulsevault") {
		final = path + ".pulsevault"
		remove = append(remove, final)
	}
	return final, remove
}

// mediaExts are carrier-friendly extensions for hide-in-picture destinations.
var mediaExts = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".gif":  {},
	".webp": {},
	".bmp":  {},
	".mp4":  {},
	".mov":  {},
	".webm": {},
}

// IsMediaPath reports whether path has a recognized image/video extension.
func IsMediaPath(path string) bool {
	_, ok := mediaExts[strings.ToLower(filepath.Ext(path))]
	return ok
}

// PrepareHidePath normalizes a hide-in-picture save path so the dest keeps a
// media extension (or .pulsevault if the user chose a vault name). Empty
// file-save placeholders (dest, and final if different) are listed for removal.
func PrepareHidePath(dest, carrier string) (final string, remove []string) {
	if dest == "" {
		return "", nil
	}
	remove = []string{dest}
	final = dest
	lower := strings.ToLower(dest)
	if IsMediaPath(dest) || strings.HasSuffix(lower, ".pulsevault") {
		return final, remove
	}
	ext := filepath.Ext(carrier)
	if ext == "" {
		return final, remove
	}
	final = dest + ext
	if final != dest {
		remove = append(remove, final)
	}
	return final, remove
}
