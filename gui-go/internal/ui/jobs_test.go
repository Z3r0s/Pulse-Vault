package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJobStateBeginFinish(t *testing.T) {
	var j JobState
	if j.Busy() {
		t.Fatal("expected idle")
	}
	if err := j.Begin("Unlock", "Deriving key…"); err != nil {
		t.Fatal(err)
	}
	if !j.Busy() {
		t.Fatal("expected busy")
	}
	busy, op, msg := j.Snapshot()
	if !busy || op != "Unlock" || msg != "Deriving key…" {
		t.Fatalf("snapshot = %v %q %q", busy, op, msg)
	}
	if err := j.Begin("Add", "nope"); err != ErrBusy {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
	j.Finish("Vault unlocked")
	if j.Busy() {
		t.Fatal("expected idle after finish")
	}
	_, _, msg = j.Snapshot()
	if msg != "Vault unlocked" {
		t.Fatalf("msg = %q", msg)
	}
}

func TestStatusLine(t *testing.T) {
	if StatusLine(false, "", "Ready") != "Ready" {
		t.Fatal("idle")
	}
	s := StatusLine(true, "Add file", "Encrypting…")
	if !strings.Contains(s, "Add file") || !strings.Contains(s, "Encrypting") {
		t.Fatalf("status = %q", s)
	}
}

func TestPrepareCreatePath(t *testing.T) {
	final := PrepareCreatePath(`C:\tmp\vault`)
	if !strings.HasSuffix(strings.ToLower(final), ".pulsevault") {
		t.Fatalf("final = %q", final)
	}
	final2 := PrepareCreatePath(`C:\tmp\x.pulsevault`)
	if final2 != `C:\tmp\x.pulsevault` {
		t.Fatalf("final2 = %q", final2)
	}
	if empty := PrepareCreatePath(""); empty != "" {
		t.Fatalf("empty path = %q", empty)
	}
}

func TestEnsureDestinationAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.pulsevault")
	if err := ensureDestinationAbsent(path); err != nil {
		t.Fatalf("new destination rejected: %v", err)
	}
	emptyPath := filepath.Join(dir, "empty.pulsevault")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureDestinationAbsent(emptyPath); err == nil {
		t.Fatal("empty existing destination should be preserved too")
	}
	if err := os.WriteFile(path, []byte("existing vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureDestinationAbsent(path); err == nil {
		t.Fatal("existing destination should be rejected")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "existing vault" {
		t.Fatalf("existing destination changed: data=%q err=%v", got, err)
	}
}

func TestOutputValidators(t *testing.T) {
	for _, valid := range []string{"New vault", "records-2026", "private.pulsevault"} {
		if err := validateOutputName(valid); err != nil {
			t.Errorf("valid name %q rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", " ", "..", `../vault`, `folder\vault`, "CON", "report?", "trailing."} {
		if err := validateOutputName(invalid); err == nil {
			t.Errorf("invalid name %q accepted", invalid)
		}
	}

	dir := t.TempDir()
	if err := validateOutputFolder(dir); err != nil {
		t.Errorf("existing directory rejected: %v", err)
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateOutputFolder(file); err == nil {
		t.Error("file path accepted as an output folder")
	}
	if err := validateOutputFolder(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing folder accepted")
	}
}

func TestPrepareHidePathKeepsMediaAndVaultExt(t *testing.T) {
	final := PrepareHidePath(`C:\tmp\out.png`, `D:\cover.jpg`)
	if final != `C:\tmp\out.png` {
		t.Fatalf("keep png = %q", final)
	}

	final = PrepareHidePath(`C:\tmp\out.JPEG`, `D:\cover.png`)
	if final != `C:\tmp\out.JPEG` {
		t.Fatalf("keep JPEG = %q", final)
	}

	final = PrepareHidePath(`C:\tmp\secret.pulsevault`, `D:\cover.png`)
	if final != `C:\tmp\secret.pulsevault` {
		t.Fatalf("keep pulsevault = %q", final)
	}

	final = PrepareHidePath(`C:\tmp\clip.mp4`, `D:\cover.png`)
	if final != `C:\tmp\clip.mp4` {
		t.Fatalf("keep mp4 = %q", final)
	}
}

func TestPrepareHidePathInheritsCarrierExt(t *testing.T) {
	final := PrepareHidePath(`C:\tmp\photo`, `D:\cover.png`)
	if final != `C:\tmp\photo.png` {
		t.Fatalf("inherit png = %q", final)
	}

	final = PrepareHidePath(`C:\tmp\movie`, `D:\clip.MP4`)
	if final != `C:\tmp\movie.MP4` {
		t.Fatalf("inherit MP4 = %q", final)
	}

	final = PrepareHidePath(`C:\tmp\notes.txt`, `D:\cover.png`)
	if final != `C:\tmp\notes.txt.png` {
		t.Fatalf("unrecognized ext = %q", final)
	}

	final = PrepareHidePath(`C:\tmp\bare`, `D:\noext`)
	if final != `C:\tmp\bare` {
		t.Fatalf("no carrier ext = %q", final)
	}

	final = PrepareHidePath("", `D:\cover.png`)
	if final != "" {
		t.Fatalf("empty dest final=%q", final)
	}
}

func TestIsMediaPath(t *testing.T) {
	yes := []string{"a.png", "B.JPG", `C:\x.jpeg`, "g.gif", "w.webp", "b.bmp", "v.mp4", "c.mov", "m.webm"}
	for _, p := range yes {
		if !IsMediaPath(p) {
			t.Fatalf("expected media: %q", p)
		}
	}
	no := []string{"vault.pulsevault", "notes.txt", "noext", "photo.png.bak", ""}
	for _, p := range no {
		if IsMediaPath(p) {
			t.Fatalf("expected non-media: %q", p)
		}
	}
}

func TestJobStateConcurrentBegin(t *testing.T) {
	var j JobState
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			errs <- j.Begin("op", "m")
		}()
	}
	var ok, busy int
	for i := 0; i < 8; i++ {
		err := <-errs
		if err == nil {
			ok++
		} else if err == ErrBusy {
			busy++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || busy != 7 {
		t.Fatalf("ok=%d busy=%d", ok, busy)
	}
	j.Finish("done")
	// ensure Finish is quick
	time.Sleep(time.Millisecond)
	if j.Busy() {
		t.Fatal("still busy")
	}
}
