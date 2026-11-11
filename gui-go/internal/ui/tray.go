package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
)

func systemTrayMenu(show, exit func()) *fyne.Menu {
	return fyne.NewMenu(AppTitle,
		fyne.NewMenuItem("Show Pulse-Vault", show),
		fyne.NewMenuItem("Exit Pulse-Vault", exit),
	)
}

func (ui *App) installSystemTray(icon fyne.Resource) {
	tray, ok := ui.fyneApp.(desktop.App)
	if !ok {
		return
	}
	tray.SetSystemTrayMenu(systemTrayMenu(ui.showFromTray, ui.exitFromTray))
	tray.SetSystemTrayIcon(icon)
	tray.SetSystemTrayWindow(ui.win)
	ui.win.SetCloseIntercept(ui.hideToTray)
}

func (ui *App) showFromTray() {
	ui.win.Show()
	ui.win.RequestFocus()
}

// hideToTray locks an idle vault before hiding its window. A running operation
// keeps the window open so the user can see when it is safe to close.
func (ui *App) hideToTray() {
	if ui.jobs.Busy() {
		dialog.ShowInformation("Operation in progress", "Wait for the current operation to finish before hiding Pulse-Vault.", ui.win)
		return
	}
	if ui.v != nil {
		ui.onLock()
	}
	ui.win.Hide()
}

func (ui *App) exitFromTray() {
	if ui.jobs.Busy() {
		ui.showFromTray()
		dialog.ShowInformation("Operation in progress", "Wait for the current operation to finish before exiting Pulse-Vault.", ui.win)
		return
	}
	if ui.v != nil {
		ui.v.Lock()
		ui.v = nil
	}
	ui.fyneApp.Quit()
}
