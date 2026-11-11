package ui

import (
	"errors"
	"fmt"
	"os"
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

// PrepareCreatePath adds the native vault extension when the user omitted it.
func PrepareCreatePath(path string) string {
	if path == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pulsevault") {
		return path + ".pulsevault"
	}
	return path
}

// ensureDestinationAbsent prevents accidental replacement of an existing file.
func ensureDestinationAbsent(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("a file already exists at %q; choose another name or move the existing file first", path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func validateOutputName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("enter a file name")
	}
	if name != strings.TrimSpace(name) || name == "." || name == ".." {
		return errors.New("remove leading or trailing spaces and choose a valid file name")
	}
	if strings.ContainsAny(name, `/\\<>:"|?*`) || strings.HasSuffix(name, ".") {
		return errors.New("use a file name without path separators or reserved characters")
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return errors.New("choose a different name; this name is reserved by Windows")
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return errors.New("choose a different name; this name is reserved by Windows")
	}
	return nil
}

func validateOutputFolder(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("choose a folder")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access folder: %w", err)
	}
	if !info.IsDir() {
		return errors.New("choose an existing folder")
	}
	return nil
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

// PrepareHidePath keeps a carrier-friendly extension on a hide-in-picture
// destination, inheriting the selected carrier extension when needed.
func PrepareHidePath(dest, carrier string) string {
	if dest == "" {
		return ""
	}
	final := dest
	lower := strings.ToLower(dest)
	if IsMediaPath(dest) || strings.HasSuffix(lower, ".pulsevault") {
		return final
	}
	ext := filepath.Ext(carrier)
	if ext == "" {
		return final
	}
	final = dest + ext
	return final
}
