package ui

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
)

func TestSystemTrayMenuOffersShowAndExit(t *testing.T) {
	showCalls, exitCalls := 0, 0
	menu := systemTrayMenu(func() { showCalls++ }, func() { exitCalls++ })
	if menu.Label != AppTitle || len(menu.Items) != 2 {
		t.Fatalf("tray menu = %#v; want app title and two actions", menu)
	}
	if menu.Items[0].Label != "Show Pulse-Vault" || menu.Items[1].Label != "Exit Pulse-Vault" {
		t.Fatalf("tray actions = %q, %q", menu.Items[0].Label, menu.Items[1].Label)
	}
	menu.Items[0].Action()
	menu.Items[1].Action()
	if showCalls != 1 || exitCalls != 1 {
		t.Fatalf("tray action calls = show:%d exit:%d", showCalls, exitCalls)
	}
}

func TestHidingWindowToTrayLocksTheVault(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(800, 600))
	v := vault.New(filepath.Join(t.TempDir(), "tray-lock.pulsevault"))
	if err := v.Create("TrayReviewPassword!!", "standard"); err != nil {
		t.Fatal(err)
	}
	ui.v = v

	ui.hideToTray()

	if v.IsUnlocked() || ui.v != nil {
		t.Fatal("hiding the window should lock and detach the unlocked vault")
	}
}
