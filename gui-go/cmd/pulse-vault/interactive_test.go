package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
)

func TestHandleConsoleListAddDeleteQuit(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "console.pulsevault")
	pw := "ConsoleHandlerPassword!!"
	v := vault.New(vp)
	if err := v.Create(pw, "fast"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(src, []byte("from-console"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := handleConsole(v, "help", bufio.NewReader(strings.NewReader(""))); err != nil {
		t.Fatalf("help: %v", err)
	}
	if err := handleConsole(v, "add "+src, bufio.NewReader(strings.NewReader(""))); err != nil {
		t.Fatalf("add: %v", err)
	}
	names, err := v.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "note.txt" {
		t.Fatalf("list after add: %v", names)
	}
	if err := handleConsole(v, "/list", bufio.NewReader(strings.NewReader(""))); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := handleConsole(v, "delete note.txt", bufio.NewReader(strings.NewReader(""))); err != nil {
		t.Fatalf("delete: %v", err)
	}
	names, err = v.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty after delete, got %v", names)
	}
	if err := handleConsole(v, "quit", bufio.NewReader(strings.NewReader(""))); err != errConsoleQuit {
		t.Fatalf("quit: %v", err)
	}
}

func TestHandleConsoleUnknown(t *testing.T) {
	dir := t.TempDir()
	v := vault.New(filepath.Join(dir, "x.pulsevault"))
	if err := v.Create("ConsoleUnknownPass!!", "fast"); err != nil {
		t.Fatal(err)
	}
	err := handleConsole(v, "nope", bufio.NewReader(strings.NewReader("")))
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("want unknown, got %v", err)
	}
}
