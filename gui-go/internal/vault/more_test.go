package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLockClearsUnlockedState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock.pulsevault")
	v := New(path)
	if err := v.Create("LockStatePassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	if !v.IsUnlocked() {
		t.Fatal("expected unlocked after create")
	}
	v.Lock()
	if v.IsUnlocked() {
		t.Fatal("expected locked")
	}
	if _, err := v.ListFiles(); err == nil {
		t.Fatal("list must fail while locked")
	}
	if err := v.Unlock("LockStatePassword!!"); err != nil {
		t.Fatal(err)
	}
	if !v.IsUnlocked() {
		t.Fatal("expected unlocked after unlock")
	}
}

func TestExtractRefusesOverwriteUnlessAsked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ow.pulsevault")
	v := New(path)
	if err := v.Create("OverwritePass1234!!", "fast"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(src, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "out")
	if _, err := v.ExtractFile("note.txt", outDir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ExtractFile("note.txt", outDir, false); err == nil {
		t.Fatal("expected already-exists")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v", err)
	}
	if _, err := v.ExtractFile("note.txt", outDir, true); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
}

func TestUnicodeFilenameRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "uni.pulsevault")
	v := New(path)
	if err := v.Create("UnicodePass1234!!", "fast"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "фото.txt")
	want := []byte("ok")
	if err := os.WriteFile(src, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	names, err := v.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "фото.txt" {
		t.Fatalf("names = %v", names)
	}
	out, err := v.ExtractFile("фото.txt", filepath.Join(dir, "out"), false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("got %q", got)
	}
}

func TestSafeFilenameRejectsTraversal(t *testing.T) {
	if _, err := safeFilename(".."); err == nil {
		t.Fatal("expected reject ..")
	}
	got, err := safeFilename(`..\secret.txt`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(got, `/\`) || got == ".." || got == "." {
		t.Fatalf("unsafe sanitized name %q", got)
	}
}

func TestEmptyPasswordRejected(t *testing.T) {
	dir := t.TempDir()
	v := New(filepath.Join(dir, "empty-pw.pulsevault"))
	if err := v.Create("", "fast"); err == nil {
		t.Fatal("create with empty password must fail")
	}
}
