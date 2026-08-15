package ui

import (
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
	final, rem := PrepareCreatePath(`C:\tmp\vault`)
	if !strings.HasSuffix(strings.ToLower(final), ".pulsevault") {
		t.Fatalf("final = %q", final)
	}
	if len(rem) < 1 {
		t.Fatal("expected removals")
	}
	final2, rem2 := PrepareCreatePath(`C:\tmp\x.pulsevault`)
	if final2 != `C:\tmp\x.pulsevault` {
		t.Fatalf("final2 = %q", final2)
	}
	if len(rem2) != 1 {
		t.Fatalf("rem2 = %v", rem2)
	}
}

func TestPrepareHidePathKeepsMediaAndVaultExt(t *testing.T) {
	final, rem := PrepareHidePath(`C:\tmp\out.png`, `D:\cover.jpg`)
	if final != `C:\tmp\out.png` {
		t.Fatalf("keep png = %q", final)
	}
	if len(rem) != 1 || rem[0] != `C:\tmp\out.png` {
		t.Fatalf("rem = %v", rem)
	}

	final, rem = PrepareHidePath(`C:\tmp\out.JPEG`, `D:\cover.png`)
	if final != `C:\tmp\out.JPEG` {
		t.Fatalf("keep JPEG = %q", final)
	}

	final, rem = PrepareHidePath(`C:\tmp\secret.pulsevault`, `D:\cover.png`)
	if final != `C:\tmp\secret.pulsevault` {
		t.Fatalf("keep pulsevault = %q", final)
	}
	if len(rem) != 1 {
		t.Fatalf("pulsevault rem = %v", rem)
	}

	final, rem = PrepareHidePath(`C:\tmp\clip.mp4`, `D:\cover.png`)
	if final != `C:\tmp\clip.mp4` {
		t.Fatalf("keep mp4 = %q", final)
	}
}

func TestPrepareHidePathInheritsCarrierExt(t *testing.T) {
	final, rem := PrepareHidePath(`C:\tmp\photo`, `D:\cover.png`)
	if final != `C:\tmp\photo.png` {
		t.Fatalf("inherit png = %q", final)
	}
	if len(rem) != 2 || rem[0] != `C:\tmp\photo` || rem[1] != `C:\tmp\photo.png` {
		t.Fatalf("rem = %v", rem)
	}

	final, rem = PrepareHidePath(`C:\tmp\movie`, `D:\clip.MP4`)
	if final != `C:\tmp\movie.MP4` {
		t.Fatalf("inherit MP4 = %q", final)
	}

	final, rem = PrepareHidePath(`C:\tmp\notes.txt`, `D:\cover.png`)
	if final != `C:\tmp\notes.txt.png` {
		t.Fatalf("unrecognized ext = %q", final)
	}

	final, rem = PrepareHidePath(`C:\tmp\bare`, `D:\noext`)
	if final != `C:\tmp\bare` {
		t.Fatalf("no carrier ext = %q", final)
	}
	if len(rem) != 1 {
		t.Fatalf("noext rem = %v", rem)
	}

	final, rem = PrepareHidePath("", `D:\cover.png`)
	if final != "" || rem != nil {
		t.Fatalf("empty dest final=%q rem=%v", final, rem)
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
