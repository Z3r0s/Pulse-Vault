package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/updatecheck"
	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
)

// TestCaptureVisualReviewScreens refreshes the screenshots used in the README
// and desktop guide. It writes only synthetic files and runs only on request.
func TestCaptureVisualReviewScreens(t *testing.T) {
	if os.Getenv("PULSEVAULT_CAPTURE_UI") != "1" {
		t.Skip("set PULSEVAULT_CAPTURE_UI=1 to refresh documented GUI captures")
	}

	a := test.NewApp()
	a.Settings().SetTheme(&PulseTheme{})
	ui := &App{fyneApp: a}
	ui.win = a.NewWindow(AppTitle)
	ui.win.SetPadded(false)
	ui.win.Resize(fyne.NewSize(1240, 800))
	ui.build()
	ui.bindShortcuts()
	ui.setLockedUI()
	ui.win.Show()
	t.Cleanup(func() {
		ui.win.Close()
		a.Quit()
	})

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate screenshot generator")
	}
	assets := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "docs", "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCapture := func(name string) {
		t.Helper()
		ui.win.Resize(ui.win.Canvas().Size())
		ui.root.Refresh()
		// The test driver does not run a desktop event loop. Refresh the virtual
		// list after its parent lays it out so visible rows are populated before
		// Canvas.Capture snapshots the frame.
		ui.fileList.Refresh()
		img := ui.win.Canvas().Capture()
		if img == nil {
			t.Fatal("capture returned no image")
		}
		path := filepath.Join(assets, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		encodeErr := png.Encode(file, img)
		closeErr := file.Close()
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}

	writeCapture("pulse-vault-desktop.png")
	_, profileChoice := ui.promptNewVaultProfile("New vault password hardening", func(string) {})
	profileChoice.SetSelected(kdfProfileOption("hardened"))
	profileChoice.OnChanged(profileChoice.Selected)
	writeCapture("pulse-vault-kdf-profile.png")
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())
	ui.win.Resize(fyne.NewSize(2560, 1440))
	writeCapture("pulse-vault-desktop-1440p.png")
	ui.win.Resize(fyne.NewSize(1240, 800))

	work := t.TempDir()
	v := vault.New(filepath.Join(work, "Example.pulsevault"))
	if err := v.Create("VisualReviewOnlyPassword!!", "standard"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Lock)
	ui.v = v
	ui.setUnlockedUI("Example.pulsevault")
	if !strings.Contains(test.RenderToMarkup(ui.win.Canvas()), "Hide in picture") {
		t.Fatal("the unlocked vault action rail should retain the Hide in picture label")
	}
	writeCapture("pulse-vault-empty.png")
	ui.onPassword()
	writeCapture("pulse-vault-change-kdf-profile.png")
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())
	ui.confirmPasswordPolicy("short", func() {})
	writeCapture("pulse-vault-password-warning.png")
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())

	files := filepath.Join(work, "sample-files")
	fixtures := map[string]string{
		"Prototype/workflow-plan.txt":  "Synthetic workflow notes for the visual review.",
		"Reference/archive-layout.svg": "Synthetic archive layout reference.",
		"Research/benchmarks.csv":      "stage,seconds\nverify,0.344\nrestore,4.482\n",
		"Research/format-notes.md":     "Synthetic notes about a sample archive format.",
	}
	for name, contents := range fixtures {
		path := filepath.Join(files, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.AddDirectory(files, false); err != nil {
		t.Fatal(err)
	}
	ui.setUnlockedUI("Example.pulsevault")
	ui.fileList.Refresh()
	markup := test.RenderToMarkup(ui.win.Canvas())
	for _, name := range []string{"Prototype/workflow-plan.txt", "Reference/archive-layout.svg", "Research/benchmarks.csv", "Research/format-notes.md"} {
		if !strings.Contains(markup, name) {
			t.Fatalf("populated workspace capture is missing file row %q", name)
		}
	}
	writeCapture("pulse-vault-files.png")

	ui.win.Resize(fyne.NewSize(640, 480))
	writeCapture("pulse-vault-compact.png")
	ui.fileList.Select(0)
	markup = test.RenderToMarkup(ui.win.Canvas())
	if !strings.Contains(markup, "Delete") || !strings.Contains(markup, "Save a copy") {
		t.Fatal("selecting a file should reveal its contextual actions in the compact sidebar")
	}
	writeCapture("pulse-vault-compact-actions.png")

	v.Lock()
	ui.v = nil
	ui.setLockedUI()
	ui.setStatus("Ready — create or open a vault.")
	ui.win.Resize(fyne.NewSize(1240, 800))
	ui.showUpdateConsent(false)
	writeCapture("pulse-vault-update-consent.png")
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())
	ui.showUpdatePrivacyInfo()
	writeCapture("pulse-vault-update-privacy.png")
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())

	assetName, supported := updatecheck.DesktopAssetName(runtime.GOOS, runtime.GOARCH)
	if !supported {
		t.Skip("no published desktop asset name for this visual capture platform")
	}
	ui.showUpdateResult(updatecheck.Release{
		TagName:         "v0.2.1",
		Name:            "Pulse-Vault 0.2.1",
		HTMLURL:         "https://github.com/Z3r0s/Pulse-Vault/releases/tag/v0.2.1",
		Body:            "Illustrative release notes. Review the linked GitHub release for the published changes.",
		CurrentVersion:  "0.2.0",
		Comparable:      true,
		UpdateAvailable: true,
		Assets: []updatecheck.Asset{
			{Name: assetName},
			{Name: "SHA256SUMS"},
		},
	})
	writeCapture("pulse-vault-update-available.png")
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())

	reviewFolder := "/tmp"
	if runtime.GOOS == "windows" {
		reviewFolder = `C:\Users\Public\Downloads`
	}
	ui.chooseOutputPath("Save verified Pulse-Vault update", "Download", assetName, reviewFolder, func(path string) string { return path }, func(string) {})
	writeCapture("pulse-vault-update-location.png")
	ui.win.Canvas().Overlays().Remove(ui.win.Canvas().Overlays().Top())

	if err := ui.jobs.Begin("Download update", "Downloading and verifying the release asset…"); err != nil {
		t.Fatal(err)
	}
	ui.setBusyChromeForOperation(true, true)
	ui.progress.SetValue(0.43)
	ui.setStatus("Download update… Downloading update · 43%")
	writeCapture("pulse-vault-update-progress.png")
	ui.jobs.Finish("Update downloaded and SHA-256 verified.")
	ui.setBusyChrome(false)

	destination := filepath.Join(reviewFolder, assetName)
	ui.setStatus("Update downloaded and SHA-256 verified.")
	ui.showVerifiedUpdateSaved(destination)
	writeCapture("pulse-vault-update-saved.png")
}
