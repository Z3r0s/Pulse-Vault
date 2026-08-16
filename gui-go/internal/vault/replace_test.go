package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFileFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "vault.pulsevault")
	if err := os.WriteFile(destination, []byte("last good vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.tmp")
	if err := replaceFile(missing, destination); err == nil {
		t.Fatal("expected replacement failure")
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "last good vault" {
		t.Fatalf("destination changed after failed replacement: %q", got)
	}
}
