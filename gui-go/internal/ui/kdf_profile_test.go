package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestNewVaultProfileDisclosureAndSelection(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	selected := ""
	dialog, choice := ui.promptNewVaultProfile("New vault password hardening", func(profile string) {
		selected = profile
	})
	markup := test.RenderToMarkup(ui.win.Canvas())
	for _, phrase := range []string{"offline password guesses", "Standard", "32 MiB"} {
		if !strings.Contains(markup, phrase) {
			t.Errorf("KDF profile prompt is missing %q", phrase)
		}
	}
	if !strings.Contains(strings.Join(choice.Options, " "), "Hardened · 1 GiB") {
		t.Fatal("the profile selector must offer the 1 GiB Hardened option")
	}
	choice.SetSelected(kdfProfileOption("hardened"))
	choice.OnChanged(choice.Selected)
	if choice.Selected != kdfProfileOption("hardened") || !strings.Contains(kdfProfileExplanation(kdfProfileFromOption(choice.Selected)), "more memory and time") {
		t.Fatal("hardened profile should explain its memory and latency tradeoff")
	}
	dialog.Confirm()
	if selected != "hardened" {
		t.Fatalf("selected profile = %q, want hardened", selected)
	}
}

func TestWeakPasswordRequiresExplicitAcknowledgment(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	proceeded := false
	dialog := ui.confirmPasswordPolicy("short", func() { proceeded = true })
	if dialog == nil || proceeded {
		t.Fatal("a weak password must pause for explicit user acknowledgment")
	}
	markup := test.RenderToMarkup(ui.win.Canvas())
	for _, phrase := range []string{"Password strength warning", "offline", "long, unique passphrase"} {
		if !strings.Contains(markup, phrase) {
			t.Errorf("password warning is missing %q", phrase)
		}
	}
	dialog.Confirm()
	if !proceeded {
		t.Fatal("Use anyway should continue only after the user confirms")
	}
}

func TestKDFProfileOptionsAreBoundToCurrentParameters(t *testing.T) {
	if got := kdfProfileOption("standard"); got != "Standard · 32 MiB" {
		t.Fatalf("standard option = %q", got)
	}
	if got := kdfProfileOption("hardened"); got != "Hardened · 1 GiB" {
		t.Fatalf("hardened option = %q", got)
	}
	if kdfProfileFromOption(kdfProfileOption("hardened")) != "hardened" {
		t.Fatal("hardened UI option did not map to its stored profile")
	}
	if kdfProfileFromOption("Fast · 16 bytes") != "" {
		t.Fatal("the test-only fast KDF profile must not appear as a UI option")
	}
}
