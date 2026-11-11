package ui

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/updatecheck"
)

type recordingUpdateService struct {
	checks atomic.Int32
	called chan struct{}
}

func (s *recordingUpdateService) Check(context.Context, string) (updatecheck.Release, error) {
	s.checks.Add(1)
	if s.called != nil {
		select {
		case s.called <- struct{}{}:
		default:
		}
	}
	return updatecheck.Release{}, errors.New("offline test transport")
}

func (*recordingUpdateService) Download(context.Context, updatecheck.Release, string, string, func(int64, int64)) error {
	return errors.New("unexpected update download in consent test")
}

func TestUpdateCheckRequiresAnExplicitSecondChoice(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	if ui.updateClient != nil {
		t.Fatal("building the app must not initialize the network updater")
	}
	service := &recordingUpdateService{}
	ui.updateClient = service
	if ui.btnCheckUpdates == nil || ui.btnCheckUpdates.Text != "Check updates" || ui.btnCheckUpdates.Disabled() {
		t.Fatal("the always-available update action should be visible and enabled")
	}
	ui.btnCheckUpdates.OnTapped()
	if ui.jobs.Busy() || service.checks.Load() != 0 {
		t.Fatal("opening the consent prompt must not contact GitHub")
	}
	if ui.btnCheckUpdates.Disabled() {
		t.Fatal("the update action should remain enabled until a request starts")
	}
}

func TestStartupAndDismissDoNotContactAnyServerUntilConsent(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	service := &recordingUpdateService{called: make(chan struct{}, 1)}
	ui.updateClient = service

	ui.promptForUpdateAtStartup()
	if ui.jobs.Busy() || service.checks.Load() != 0 {
		t.Fatal("launching the startup prompt must not make a network request")
	}

	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())
	consent := ui.showUpdateConsent(false)
	if consent == nil {
		t.Fatal("expected manual update consent dialog")
	}
	consent.Dismiss()
	if ui.jobs.Busy() || service.checks.Load() != 0 {
		t.Fatal("choosing Not now must leave the app offline")
	}

	consent = ui.showUpdateConsent(false)
	if consent == nil {
		t.Fatal("expected a second manual update consent dialog")
	}
	consent.Confirm()
	select {
	case <-service.called:
	case <-time.After(2 * time.Second):
		t.Fatal("Check now did not start the explicitly requested release check")
	}
	if got := service.checks.Load(); got != 1 {
		t.Fatalf("Check now issued %d release checks, want exactly one", got)
	}
}

func TestStartupUpdatePromptCanBeSuppressed(t *testing.T) {
	ui, app := testUI(t, fyne.NewSize(1120, 700))
	service := &recordingUpdateService{}
	ui.updateClient = service
	ui.promptForUpdateAtStartup()
	if ui.jobs.Busy() || service.checks.Load() != 0 {
		t.Fatal("showing the startup consent prompt must not make a network request")
	}
	if len(ui.win.Canvas().Overlays().List()) == 0 {
		t.Fatal("the update consent prompt should appear by default on startup")
	}
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())
	app.Preferences().SetBool(updatePromptPreference, false)
	ui.promptForUpdateAtStartup()
	if len(ui.win.Canvas().Overlays().List()) != 0 {
		t.Fatal("the startup update prompt should stay hidden after the user disables it")
	}
}

func TestUpdatePrivacyDisclosureExplainsNetworkExposure(t *testing.T) {
	for _, phrase := range []string{"GitHub", "source IP", "VPN", "HTTPS proxy", "vault contents", "Check now"} {
		if !strings.Contains(updatePrivacySummary, phrase) {
			t.Errorf("update disclosure is missing %q", phrase)
		}
	}
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	ui.showUpdatePrivacyInfo()
	markup := test.RenderToMarkup(ui.win.Canvas())
	for _, phrase := range []string{"NO_PROXY", "platform-specific release asset", "SHA256SUMS", "never installs", "browser's network settings"} {
		if !strings.Contains(markup, phrase) {
			t.Errorf("detailed update disclosure is missing %q", phrase)
		}
	}
}

func TestShortReleaseNotesAreBoundedAndReadable(t *testing.T) {
	if got := shortReleaseNotes("  Fixes and improvements.\n "); got != "Fixes and improvements." {
		t.Fatalf("short notes = %q", got)
	}
	long := strings.Repeat("release note ", 100)
	got := shortReleaseNotes(long)
	if len(got) > 604 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long notes should be truncated: len=%d suffix=%q", len(got), got[max(0, len(got)-4):])
	}
}

func TestVerifiedUpdateMessageNamesTheChosenDestinationAndManualInstall(t *testing.T) {
	message := verifiedUpdateMessage(`C:\Users\you\Downloads\pulse-vault-gui-windows-amd64.exe`)
	for _, phrase := range []string{"C:\\Users\\you\\Downloads", "matches the SHA256SUMS", "has not been installed", "keep a backup"} {
		if !strings.Contains(message, phrase) {
			t.Errorf("verified update message is missing %q", phrase)
		}
	}
}
