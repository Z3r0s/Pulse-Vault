package ui

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/updatecheck"
	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/version"
)

const updatePromptPreference = "updates.ask-on-startup"

const updatePrivacySummary = "Optional: GitHub sees the source IP address and request time when you choose Check now. A VPN or HTTPS proxy can change the address GitHub sees. No vault contents, names, or password are sent."

type updateService interface {
	Check(context.Context, string) (updatecheck.Release, error)
	Download(context.Context, updatecheck.Release, string, string, func(done, total int64)) error
}

func (ui *App) promptForUpdateAtStartup() {
	if ui.fyneApp == nil || !ui.fyneApp.Preferences().BoolWithFallback(updatePromptPreference, true) {
		return
	}
	ui.showUpdateConsent(true)
}

func (ui *App) onCheckUpdates() {
	ui.showUpdateConsent(false)
}

func (ui *App) showUpdateConsent(startup bool) *dialog.ConfirmDialog {
	if ui.jobs.Busy() {
		ui.setStatus("Wait for the current operation to finish before checking for updates.")
		return nil
	}
	question := widget.NewLabelWithStyle("Would you like to check for updates now?", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	info := widget.NewButtonWithIcon("Privacy info", theme.InfoIcon(), func() {
		ui.showUpdatePrivacyInfo()
	})
	info.Importance = widget.LowImportance
	privacy := widget.NewLabel(updatePrivacySummary)
	privacy.Wrapping = fyne.TextWrapWord
	privacy.Importance = widget.LowImportance
	dontAsk := widget.NewCheck("Don't ask me on startup", nil)
	content := container.NewVBox(
		container.NewHBox(question, info),
		privacy,
		dontAsk,
	)
	confirm := dialog.NewCustomConfirm("Check for updates?", "Check now", "Not now", content, func(check bool) {
		ui.fyneApp.Preferences().SetBool(updatePromptPreference, !dontAsk.Checked)
		if check {
			ui.checkForUpdates()
		} else if !startup {
			ui.setStatus("Update check skipped. No connection was made.")
		}
	}, ui.win)
	confirm.Resize(fyne.NewSize(560, 270))
	confirm.Show()
	return confirm
}

func (ui *App) showUpdatePrivacyInfo() {
	connection := "No request is made until you choose Check now. That request contacts api.github.com. GitHub can see the source IP address (normally your public IP), request time, and request details. A VPN changes the route; Go's HTTP client reads HTTPS_PROXY or https_proxy to use an HTTPS proxy, unless api.github.com is excluded by NO_PROXY. Configure your VPN or proxy before starting Pulse-Vault. The VPN or proxy provider and your network may also see connection metadata."
	download := "The check sends no vault contents, file names, or password. Choosing a download makes a separate request for the platform-specific release asset, which tells GitHub which asset you selected. The file is saved to a folder you choose. Pulse-Vault checks it against SHA256SUMS from the same GitHub release; this detects a content or transfer mismatch but does not independently authenticate the publisher. The app never installs or replaces itself automatically. Opening release notes in a browser is another GitHub request governed by your browser's network settings."
	content := container.NewVBox(
		widget.NewLabelWithStyle("Before you check", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		wrappedLabel(connection),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("If you download", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		wrappedLabel(download),
	)
	info := dialog.NewCustom("Update check and connection privacy", "Close", container.NewScroll(content), ui.win)
	info.Resize(fyne.NewSize(700, 480))
	info.Show()
}

func (ui *App) checkForUpdates() {
	if ui.updateClient == nil {
		ui.updateClient = updatecheck.New()
	}
	var release updatecheck.Release
	ui.runAsyncWithOptions("Check for updates", "Requesting the latest release from GitHub…", func(ctx context.Context, _ func(vault.OperationUpdate)) error {
		latest, err := ui.updateClient.Check(ctx, version.Version)
		if err != nil {
			return err
		}
		release = latest
		return nil
	}, func() {
		ui.showUpdateResult(release)
	})
}

func (ui *App) showUpdateResult(release updatecheck.Release) {
	if release.Comparable && !release.UpdateAvailable {
		message := fmt.Sprintf("You're running Pulse-Vault %s. The latest stable release is %s.", release.CurrentVersion, release.TagName)
		if compared, err := updatecheck.CompareVersions(release.CurrentVersion, release.TagName); err == nil && compared > 0 {
			message = fmt.Sprintf("You're running Pulse-Vault %s, which is newer than the latest published release (%s).", release.CurrentVersion, release.TagName)
		} else {
			message += " No update is needed."
		}
		dialog.ShowInformation("Pulse-Vault is up to date", message, ui.win)
		ui.setStatus("Update check complete — no newer release found.")
		return
	}

	message := fmt.Sprintf("The latest stable release is %s.", release.TagName)
	if release.Comparable {
		message = fmt.Sprintf("Pulse-Vault %s is available. You're running %s.", release.TagName, release.CurrentVersion)
	} else {
		message += " This development build has no release version to compare, so you can choose whether to download the published build."
	}
	if notes := shortReleaseNotes(release.Body); notes != "" {
		message += "\n\n" + notes
	}
	pageURL, _ := url.Parse(release.HTMLURL) // Check validates this as the Pulse-Vault GitHub release page.
	viewRelease := widget.NewHyperlink("View release notes on GitHub", pageURL)
	assetName, supported := updatecheck.DesktopAssetName(runtime.GOOS, runtime.GOARCH)
	_, hasAsset := release.FindAsset(assetName)
	_, hasChecksums := release.FindAsset("SHA256SUMS")
	if !supported || !hasAsset || !hasChecksums {
		content := container.NewVBox(
			wrappedLabel(message),
			wrappedLabel("A verified desktop download for this operating system and architecture isn't available in this release."),
			viewRelease,
		)
		dialog.ShowCustom("Latest release found", "Close", content, ui.win)
		ui.setStatus("Latest release found; no verified desktop asset is available for this platform.")
		return
	}

	content := container.NewVBox(
		wrappedLabel(message),
		widget.NewSeparator(),
		wrappedLabel("Choose whether to download the matching desktop build. Pulse-Vault checks its SHA-256 against SHA256SUMS from the same GitHub release. This catches a mismatch but is not an independent publisher signature. Downloading will not replace or restart the running app."),
		viewRelease,
	)
	confirm := dialog.NewCustomConfirm("Pulse-Vault update available", "Choose download location…", "Not now", content, func(download bool) {
		if !download {
			ui.setStatus("Update download skipped.")
			return
		}
		ui.chooseUpdateDestination(release, assetName)
	}, ui.win)
	confirm.Resize(fyne.NewSize(620, 420))
	confirm.Show()
	ui.setStatus("A newer Pulse-Vault release is available.")
}

func (ui *App) chooseUpdateDestination(release updatecheck.Release, assetName string) {
	folder, err := os.UserHomeDir()
	if err != nil {
		folder = "."
	}
	ui.chooseOutputPath("Save verified Pulse-Vault update", "Download", assetName, folder, func(path string) string { return path }, func(destination string) {
		ui.downloadUpdate(release, assetName, destination)
	})
}

func (ui *App) downloadUpdate(release updatecheck.Release, assetName, destination string) {
	if ui.updateClient == nil {
		ui.updateClient = updatecheck.New()
	}
	ui.runAsyncWithOptions("Download update", "Downloading and verifying the release asset…", func(ctx context.Context, report func(vault.OperationUpdate)) error {
		return ui.updateClient.Download(ctx, release, assetName, destination, func(done, total int64) {
			report(vault.OperationUpdate{Phase: "Downloading update", Done: done, Total: total})
		})
	}, func() {
		ui.setStatus("Update downloaded and SHA-256 verified.")
		ui.showVerifiedUpdateSaved(destination)
	})
}

func (ui *App) showVerifiedUpdateSaved(destination string) {
	dialog.ShowInformation("Verified update saved", verifiedUpdateMessage(destination), ui.win)
}

func verifiedUpdateMessage(destination string) string {
	return fmt.Sprintf("The downloaded file matches the SHA256SUMS entry from this release.\n\nSaved to:\n%s\n\nIt has not been installed. Close Pulse-Vault before replacing the current app, and keep a backup of the existing file.", destination)
}

func wrappedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

func shortReleaseNotes(notes string) string {
	var summary strings.Builder
	inCode := false
	for _, line := range strings.Split(notes, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			continue
		}
		if !inCode {
			summary.WriteString(line)
			summary.WriteByte('\n')
		}
	}
	runes := []rune(strings.TrimSpace(summary.String()))
	truncated := len(runes) > 600
	if truncated {
		runes = runes[:600]
	}
	var clean strings.Builder
	for _, r := range runes {
		if r == '\n' || r == '\t' || r >= 0x20 {
			clean.WriteRune(r)
		}
	}
	value := strings.TrimSpace(clean.String())
	if value != "" && truncated {
		value += "…"
	}
	return value
}
