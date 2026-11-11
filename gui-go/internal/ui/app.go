// Package ui is the fully custom Pulse-Vault desktop shell.
// Long vault ops run off the UI thread; results apply via fyne.Do (main runtime).
package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/version"
)

const AppTitle = "Pulse-Vault"

// App is the multi-pane vault desktop UI.
type App struct {
	fyneApp      fyne.App
	win          fyne.Window
	v            *vault.Vault
	jobs         JobState
	updateClient updateService

	pathLabel         *widget.Label
	status            *widget.Label
	progress          *widget.ProgressBar
	cancelJob         context.CancelFunc
	jobDone           <-chan struct{}
	fileList          *widget.List
	files             []string
	fileSummaries     map[string]vault.FileSummary
	searchNames       []string
	visibleFiles      []string
	search            *widget.Entry
	noSearchResults   *widget.Label
	selected          string
	heroCard          *fyne.Container
	heroTitle         *canvas.Text
	heroBody          *widget.Label
	heroLogo          *canvas.Image
	heroActions       *fyne.Container
	heroLockedActions *fyne.Container
	heroEmptyActions  *fyne.Container
	heroAddFile       *widget.Button
	heroAddFolder     *widget.Button
	listHeader        *widget.Label
	countLabel        *widget.Label
	lockBadge         *canvas.Text
	lockBadgeBg       *canvas.Rectangle
	brandMark         *canvas.Image
	brandTitle        *canvas.Text
	brandSub          *canvas.Text
	sectionVault      *widget.Label
	sectionFiles      *widget.Label
	fileNav           *fyne.Container
	versionChip       *canvas.Text
	accentRule        *canvas.Rectangle
	busyPip           *canvas.Rectangle
	split             *container.Split
	navRail           *fyne.Container
	topBar            *fyne.Container
	mainStage         *fyne.Container
	root              *fyne.Container

	btnCreate           *widget.Button
	btnOpen             *widget.Button
	btnHide             *widget.Button
	compactVaultActions *fyne.Container
	fileAddRow          *fyne.Container
	fileSaveRow         *fyne.Container
	fileSecurityRow     *fyne.Container
	navScroll           *container.Scroll
	shortcutGuide       *widget.Label
	btnAdd              *widget.Button
	btnAddFolder        *widget.Button
	btnExtract          *widget.Button
	btnExtractAll       *widget.Button
	btnCancel           *widget.Button
	btnDelete           *widget.Button
	btnPassword         *widget.Button
	btnLock             *widget.Button
	btnCheckUpdates     *widget.Button
}

// Run launches the desktop window (blocks until closed).
func Run(a fyne.App) {
	ui := &App{fyneApp: a}
	mark := brandResource()
	a.SetIcon(mark)
	ui.win = a.NewWindow(AppTitle + "  ·  " + version.Version)
	ui.win.SetIcon(mark)
	ui.win.SetPadded(false)
	ui.win.Resize(fyne.NewSize(1240, 800))
	ui.win.SetFixedSize(false)
	ui.win.SetMaster()
	ui.build()
	ui.bindShortcuts()
	ui.win.SetOnDropped(ui.onDropped)
	ui.setLockedUI()
	a.Lifecycle().SetOnStarted(func() {
		ui.installSystemTray(mark)
		ui.promptForUpdateAtStartup()
	})
	ui.win.ShowAndRun()
}

func (ui *App) build() {
	ui.brandMark = newBrandImage(52)

	ui.brandTitle = canvas.NewText("PULSE-VAULT", colFg)
	ui.brandTitle.TextStyle = fyne.TextStyle{Bold: true}
	ui.brandTitle.TextSize = textSizeBrandTitle

	ui.brandSub = canvas.NewText("DNSPulse  ·  dnspulse.org  ·  offline", colMuted)
	ui.brandSub.TextSize = textSizeBrandSubtitle

	versionLabel := "v" + version.Version
	if version.Version == "dev" {
		versionLabel = "DEV"
	}
	ui.versionChip = canvas.NewText("  "+versionLabel+"  ", colPrimary)
	ui.versionChip.TextSize = textSizeVersionChip
	ui.versionChip.TextStyle = fyne.TextStyle{Bold: true}
	ui.btnCheckUpdates = widget.NewButtonWithIcon("Check updates", theme.DownloadIcon(), ui.onCheckUpdates)
	ui.btnCheckUpdates.Importance = widget.LowImportance

	ui.lockBadge = canvas.NewText("  ●  LOCKED  ", colLockRed)
	ui.lockBadge.TextStyle = fyne.TextStyle{Bold: true}
	ui.lockBadge.TextSize = textSizeLockBadge

	ui.pathLabel = widget.NewLabel("No vault selected")
	ui.pathLabel.Truncation = fyne.TextTruncateEllipsis
	ui.pathLabel.Importance = widget.LowImportance

	ui.status = widget.NewLabel("Ready — create or open a vault.")
	ui.status.Wrapping = fyne.TextWrapWord
	ui.progress = widget.NewProgressBar()
	ui.progress.Hide()
	ui.btnCancel = widget.NewButton("Cancel", ui.cancelOperation)
	ui.btnCancel.Importance = widget.LowImportance
	ui.btnCancel.Hide()

	ui.busyPip = canvas.NewRectangle(colPrimary)
	ui.busyPip.CornerRadius = 4
	ui.busyPip.SetMinSize(fyne.NewSize(8, 8))
	ui.busyPip.Hide()

	ui.listHeader = widget.NewLabelWithStyle("Encrypted files", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	ui.countLabel = widget.NewLabel("0 items")
	ui.countLabel.Importance = widget.LowImportance

	ui.sectionVault = widget.NewLabel("VAULT")
	ui.sectionVault.Importance = widget.MediumImportance
	ui.sectionFiles = widget.NewLabel("FILES")
	ui.sectionFiles.Importance = widget.MediumImportance

	ui.files = nil
	ui.fileList = widget.NewList(
		func() int { return len(ui.visibleFiles) },
		newFileListRow,
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := obj.(*fileListRow)
			name := ui.visibleFiles[id]
			details := ""
			if summary, ok := ui.fileSummaries[name]; ok {
				details = formatFileSummary(summary)
			}
			updateFileListRow(row, name, details)
		},
	)
	ui.fileList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(ui.visibleFiles) {
			ui.selected = ui.visibleFiles[id]
			ui.setFileActionAvailability()
			if !ui.jobs.Busy() {
				ui.setStatus("Selected · " + ui.selected)
			}
		}
	}
	ui.search = widget.NewEntry()
	ui.search.SetPlaceHolder("Find a file by name or path")
	ui.search.OnChanged = func(string) { ui.filterFiles() }
	ui.search.Hide()
	ui.noSearchResults = widget.NewLabel("No files match your search.")
	ui.noSearchResults.Alignment = fyne.TextAlignCenter
	ui.noSearchResults.Hide()

	ui.heroTitle = canvas.NewText("Get started", colFg)
	ui.heroTitle.TextStyle = fyne.TextStyle{Bold: true}
	ui.heroTitle.TextSize = textSizeHeroTitle
	ui.heroTitle.Alignment = fyne.TextAlignCenter
	ui.heroBody = widget.NewLabel(
		"Create or open a vault. Adding files copies them; originals stay in place.",
	)
	ui.heroBody.Alignment = fyne.TextAlignCenter
	ui.heroBody.Wrapping = fyne.TextWrapWord
	ui.heroLogo = newHeroMark()
	heroNew := widget.NewButton("Create vault", ui.onCreate)
	heroNew.Importance = widget.HighImportance
	heroOpen := widget.NewButton("Open vault", ui.onOpen)
	ui.heroLockedActions = container.NewCenter(container.NewHBox(heroNew, heroOpen))
	ui.heroAddFile = widget.NewButton("Add file", ui.onAdd)
	ui.heroAddFile.Importance = widget.HighImportance
	ui.heroAddFolder = widget.NewButton("Add folder", ui.onAddFolder)
	ui.heroEmptyActions = container.NewCenter(container.NewHBox(ui.heroAddFile, ui.heroAddFolder))
	ui.heroEmptyActions.Hide()
	ui.heroActions = container.NewStack(ui.heroLockedActions, ui.heroEmptyActions)
	heroInner := container.New(&heroContentLayout{}, ui.heroLogo, ui.heroTitle, ui.heroBody, ui.heroActions)
	heroBg := canvas.NewRectangle(colCard)
	heroBg.CornerRadius = 12
	ui.heroCard = container.NewStack(
		heroBg,
		container.NewPadded(heroInner),
	)

	listPane := container.NewStack(ui.heroCard, ui.fileList, container.NewCenter(ui.noSearchResults))
	ui.fileList.Hide()

	ui.btnCreate = widget.NewButtonWithIcon("  Create vault", theme.DocumentCreateIcon(), ui.onCreate)
	ui.btnCreate.Importance = widget.HighImportance
	ui.btnOpen = widget.NewButtonWithIcon("  Open vault", theme.FolderOpenIcon(), ui.onOpen)
	ui.btnHide = widget.NewButtonWithIcon("  Hide in picture", theme.MediaPhotoIcon(), ui.onHide)
	ui.compactVaultActions = container.NewVBox(ui.btnCreate, ui.btnOpen, ui.btnHide)
	ui.btnAdd = widget.NewButtonWithIcon("  Add file", theme.ContentAddIcon(), ui.onAdd)
	ui.btnAddFolder = widget.NewButtonWithIcon("  Add folder", theme.FolderOpenIcon(), ui.onAddFolder)
	ui.btnExtract = widget.NewButtonWithIcon("  Save a copy", theme.DownloadIcon(), ui.onExtract)
	ui.btnExtractAll = widget.NewButton("  Save all files", ui.onExtractAll)
	ui.btnDelete = widget.NewButtonWithIcon("  Delete", theme.DeleteIcon(), ui.onDelete)
	ui.btnPassword = widget.NewButtonWithIcon("  Change password", theme.LoginIcon(), ui.onPassword)
	ui.btnLock = widget.NewButtonWithIcon("  Lock vault", theme.LogoutIcon(), ui.onLock)
	ui.fileAddRow = container.NewGridWithColumns(2, ui.btnAdd, ui.btnAddFolder)
	ui.fileSaveRow = container.NewGridWithColumns(2, ui.btnExtract, ui.btnExtractAll)
	ui.fileSecurityRow = container.NewGridWithColumns(2, ui.btnPassword, ui.btnLock)

	accentBar := canvas.NewRectangle(colPrimary)
	accentBar.CornerRadius = 2
	accentBar.SetMinSize(fyne.NewSize(4, 40))
	titleBlock := container.NewVBox(ui.brandTitle, ui.brandSub)
	topLeft := container.NewHBox(
		ui.brandMark,
		accentBar,
		titleBlock,
	)
	chipBg := canvas.NewRectangle(colCard)
	chipBg.CornerRadius = 10
	ui.lockBadgeBg = canvas.NewRectangle(colSidebar)
	ui.lockBadgeBg.CornerRadius = 10
	topRight := container.NewHBox(
		ui.btnCheckUpdates,
		container.NewStack(chipBg, container.NewPadded(ui.versionChip)),
		widget.NewLabel(" "),
		container.NewStack(ui.lockBadgeBg, container.NewPadded(ui.lockBadge)),
	)
	topBarBg := canvas.NewRectangle(colTopBar)
	ui.accentRule = canvas.NewRectangle(colPrimary)
	ui.accentRule.SetMinSize(fyne.NewSize(1, 2))
	topBarInner := container.New(&headerWrapLayout{}, topLeft, topRight)
	ui.topBar = container.NewBorder(
		nil, ui.accentRule, nil, nil,
		container.NewStack(topBarBg, container.NewPadded(topBarInner)),
	)

	privacy := canvas.NewText("dnspulse.org  ·  No cloud  ·  No telemetry", colMuted)
	privacy.TextSize = textSizePrivacyNote
	privacyNote := container.NewPadded(privacy)
	ui.shortcutGuide = widget.NewLabel("")
	ui.shortcutGuide.Importance = widget.LowImportance
	ui.fileNav = container.NewVBox(
		widget.NewSeparator(),
		ui.sectionFiles,
		ui.fileAddRow,
		ui.fileSaveRow,
		ui.btnDelete,
		widget.NewSeparator(),
		ui.fileSecurityRow,
	)
	ui.fileNav.Hide()
	nav := container.NewVBox(
		ui.sectionVault,
		ui.compactVaultActions,
		ui.fileNav,
		ui.shortcutGuide,
		privacyNote,
	)
	railAccent := canvas.NewRectangle(colPrimary)
	railAccent.SetMinSize(fyne.NewSize(3, 1))
	railBg := canvas.NewRectangle(colSidebar)
	navFloor := canvas.NewRectangle(colTransparent)
	navFloor.SetMinSize(fyne.NewSize(navTargetWidth, 1))
	ui.navScroll = container.NewVScroll(container.NewPadded(nav))
	ui.navRail = container.NewStack(
		railBg,
		navFloor,
		container.NewBorder(nil, nil, railAccent, nil, ui.navScroll),
	)

	headerRow := container.NewBorder(nil, nil, ui.listHeader, ui.countLabel)
	stageHeader := container.NewVBox(
		headerRow,
		ui.pathLabel,
		ui.search,
		widget.NewSeparator(),
	)
	stageBg := canvas.NewRectangle(colPanel)
	stageInner := container.New(
		&stageContentLayout{pathLabel: ui.pathLabel, compactHeight: compactPathCollapseHeight},
		container.NewPadded(stageHeader),
		listPane,
	)
	ui.mainStage = container.NewStack(stageBg, stageInner)

	statusBg := canvas.NewRectangle(colTopBar)
	statusRow := container.NewBorder(nil, nil, ui.busyPip, ui.btnCancel, ui.status)
	statusInner := container.NewVBox(ui.progress, statusRow)
	statusDock := container.NewStack(statusBg, container.NewPadded(statusInner))

	ui.split = container.NewHSplit(ui.navRail, ui.mainStage)
	ui.split.Offset = 0.24
	fitted := container.New(&splitFitLayout{
		compactHeight: compactNavHeight,
		compactHide:   []fyne.CanvasObject{ui.shortcutGuide, privacyNote},
		compactChange: ui.setCompactSidebar,
		refresh:       ui.navScroll.Refresh,
	}, ui.split)
	ui.root = container.NewBorder(ui.topBar, statusDock, nil, nil, fitted)
	ui.win.SetContent(ui.root)
}

// fileListRow is a widget so Fyne's renderer paints its nested container in
// both the desktop app and the screenshot/test drivers.
type fileListRow struct {
	widget.BaseWidget
	content *fyne.Container
	name    *widget.Label
	details *canvas.Text
}

func newFileListRow() fyne.CanvasObject {
	icon := widget.NewIcon(theme.FileIcon())
	name := widget.NewLabel("filename.ext")
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.Wrapping = fyne.TextWrapOff
	name.Truncation = fyne.TextTruncateEllipsis
	details := canvas.NewText(" ", colMuted)
	details.TextSize = (&PulseTheme{}).Size(theme.SizeNameCaptionText)
	// Keep metadata away from the window edge so its final characters stay visible.
	trailingGutter := canvas.NewRectangle(colTransparent)
	trailingGutter.SetMinSize(fyne.NewSize(12, 1))
	trailing := container.NewHBox(details, trailingGutter)
	row := &fileListRow{
		content: container.NewBorder(nil, nil, icon, trailing, name),
		name:    name,
		details: details,
	}
	row.ExtendBaseWidget(row)
	return row
}

func (row *fileListRow) CreateRenderer() fyne.WidgetRenderer {
	row.ExtendBaseWidget(row)
	return widget.NewSimpleRenderer(row.content)
}

func updateFileListRow(row *fileListRow, name, details string) {
	// Update both children before refreshing the container once. Refreshing
	// the label and metadata separately would repaint the same virtual row
	// multiple times during a list scroll or search.
	row.name.Text = name
	if details == "" {
		details = " "
	}
	row.details.Text = details
	row.Refresh()
}

func (ui *App) setCompactSidebar(compact bool) {
	if compact {
		ui.compactVaultActions.Layout = layout.NewGridLayoutWithColumns(3)
		ui.btnCreate.SetText("New")
		ui.btnCreate.SetIcon(nil)
		ui.btnOpen.SetText("Open")
		ui.btnOpen.SetIcon(nil)
		ui.btnHide.SetText("Hide")
		ui.btnHide.SetIcon(nil)
		ui.btnAdd.SetText("Add file")
		ui.btnAdd.SetIcon(nil)
		ui.btnAddFolder.SetText("Add folder")
		ui.btnAddFolder.SetIcon(nil)
		ui.btnExtract.SetText("Save a copy")
		ui.btnExtract.SetIcon(nil)
		ui.btnExtractAll.SetText("Save all")
		ui.btnDelete.SetText("Delete")
		ui.btnDelete.SetIcon(nil)
		ui.btnPassword.SetText("Password")
		ui.btnPassword.SetIcon(nil)
		ui.btnLock.SetText("Lock")
		ui.btnLock.SetIcon(nil)
	} else {
		ui.compactVaultActions.Layout = layout.NewVBoxLayout()
		ui.btnCreate.SetText("  Create vault")
		ui.btnCreate.SetIcon(theme.DocumentCreateIcon())
		ui.btnOpen.SetText("  Open vault")
		ui.btnOpen.SetIcon(theme.FolderOpenIcon())
		ui.btnHide.SetText("  Hide in picture")
		ui.btnHide.SetIcon(theme.MediaPhotoIcon())
		ui.btnAdd.SetText("  Add file")
		ui.btnAdd.SetIcon(theme.ContentAddIcon())
		ui.btnAddFolder.SetText("  Add folder")
		ui.btnAddFolder.SetIcon(theme.FolderOpenIcon())
		ui.btnExtract.SetText("  Save a copy")
		ui.btnExtract.SetIcon(theme.DownloadIcon())
		ui.btnExtractAll.SetText("  Save all files")
		ui.btnDelete.SetText("  Delete")
		ui.btnDelete.SetIcon(theme.DeleteIcon())
		ui.btnPassword.SetText("  Change password")
		ui.btnPassword.SetIcon(theme.LoginIcon())
		ui.btnLock.SetText("  Lock vault")
		ui.btnLock.SetIcon(theme.LogoutIcon())
	}
	ui.compactVaultActions.Refresh()
	ui.fileNav.Refresh()
}

func (ui *App) refreshShortcutGuide() {
	if ui.shortcutGuide == nil {
		return
	}
	lines := []string{"Ctrl+N  New vault", "Ctrl+O  Open vault"}
	if ui.v != nil && ui.v.IsUnlocked() {
		if len(ui.files) > 0 {
			lines = append(lines,
				"Ctrl+F  Find file",
				"Ctrl+E  Save selected",
				"Ctrl+Shift+S  Save all",
				"Delete  Remove selected",
			)
		}
		lines = append(lines, "Ctrl+L  Lock vault")
	}
	ui.shortcutGuide.SetText(strings.Join(lines, "\n"))
}

func formatFileSummary(summary vault.FileSummary) string {
	details := formatBytes(summary.Size)
	if summary.AddedAt > 0 {
		details += " · Added " + time.Unix(summary.AddedAt, 0).Local().Format("Jan 2")
	}
	return details
}

func (ui *App) bindShortcuts() {
	c := ui.win.Canvas()
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) {
		if ui.canUseShortcut(false) {
			ui.onCreate()
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) {
		if ui.canUseShortcut(false) {
			ui.onOpen()
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyL, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) {
		if ui.canUseShortcut(false) {
			ui.onLock()
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyF, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) {
		if ui.search.Visible() && ui.canUseShortcut(true) {
			c.Focus(ui.search)
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyE, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) {
		if ui.canUseShortcut(false) {
			ui.onExtract()
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	}, func(fyne.Shortcut) {
		if ui.canUseShortcut(false) {
			ui.onExtractAll()
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyDelete,
	}, func(fyne.Shortcut) {
		if ui.canUseShortcut(false) {
			ui.onDelete()
		}
	})
}

// canUseShortcut keeps global commands from firing while the user is typing,
// a dialog is open, or a long-running operation owns the workspace.
func (ui *App) canUseShortcut(allowSearchFocus bool) bool {
	if ui.jobs.Busy() || ui.win == nil {
		return false
	}
	c := ui.win.Canvas()
	if c.Overlays().Top() != nil {
		return false
	}
	if entry, ok := c.Focused().(*widget.Entry); ok {
		return allowSearchFocus && entry == ui.search
	}
	return true
}

func (ui *App) onDropped(_ fyne.Position, uris []fyne.URI) {
	if ui.jobs.Busy() || len(uris) == 0 {
		return
	}
	if ui.v != nil && ui.v.IsUnlocked() {
		paths := make([]string, 0, len(uris))
		for _, uri := range uris {
			if path := uri.Path(); path != "" {
				paths = append(paths, path)
			}
		}
		ui.startAdd(paths, "dropped selection")
		return
	}
	path := uris[0].Path()
	if path == "" {
		return
	}
	ui.promptPassword("Unlock dropped vault", false, func(pw string) {
		if pw == "" {
			return
		}
		var unlocked *vault.Vault
		ui.runAsync("Unlock", "Deriving key & decrypting metadata…", func() error {
			v := vault.New(path)
			if err := v.Unlock(pw); err != nil {
				return fmt.Errorf("unlock failed: wrong password, corrupted vault, or not a vault — you can open a .pulsevault or a picture/video with a hidden vault")
			}
			unlocked = v
			return nil
		}, func() {
			if ui.v != nil {
				ui.v.Lock()
			}
			ui.v = unlocked
			ui.setUnlockedUI(path)
			msg := fmt.Sprintf("Unlocked %s", filepath.Base(path))
			if unlocked.HasCarrier() {
				msg = fmt.Sprintf("Unlocked %s · Hidden inside picture · %d byte prefix", filepath.Base(path), unlocked.CarrierPrefix())
			}
			ui.jobs.Finish(msg)
			ui.setStatus(msg)
		})
	})
}

func (ui *App) syncStage() {
	// Visibility changes alter the minimum sizes of the navigation and
	// workspace headers. Relayout their parents before drawing the new state.
	defer func() {
		if ui.navRail != nil {
			ui.navRail.Refresh()
		}
		if ui.mainStage != nil {
			ui.mainStage.Refresh()
		}
	}()
	unlocked := ui.v != nil && ui.v.IsUnlocked()
	ui.syncNavigation(unlocked)
	if unlocked && len(ui.files) > 0 {
		ui.search.Show()
	} else {
		ui.search.Hide()
	}
	if !unlocked {
		ui.noSearchResults.Hide()
		ui.heroTitle.Text = "Get started"
		ui.heroTitle.Refresh()
		ui.heroBody.SetText("Create or open a vault. Adding files copies them; originals stay in place.")
		ui.heroLockedActions.Show()
		ui.heroEmptyActions.Hide()
		ui.heroActions.Show()
		ui.heroCard.Show()
		ui.fileList.Hide()
		return
	}
	if len(ui.files) == 0 {
		ui.noSearchResults.Hide()
		ui.heroTitle.Text = "Your vault is ready"
		ui.heroTitle.Refresh()
		ui.heroBody.SetText("It’s empty. Adding a file or folder copies it in; the original stays where it is.")
		ui.heroLockedActions.Hide()
		ui.heroEmptyActions.Show()
		ui.heroActions.Show()
		ui.heroCard.Show()
		ui.fileList.Hide()
		return
	}
	ui.heroActions.Hide()
	ui.heroCard.Hide()
	if len(ui.visibleFiles) == 0 {
		ui.fileList.Hide()
		ui.noSearchResults.Show()
	} else {
		ui.noSearchResults.Hide()
		wasHidden := !ui.fileList.Visible()
		ui.fileList.Show()
		if wasHidden {
			ui.fileList.Refresh()
		}
	}
}

func (ui *App) syncNavigation(unlocked bool) {
	if ui.fileNav == nil {
		return
	}
	if !unlocked {
		ui.fileNav.Hide()
		return
	}
	ui.fileNav.Show()
	if len(ui.files) == 0 {
		ui.btnExtract.Hide()
		ui.btnExtractAll.Hide()
		ui.fileSaveRow.Hide()
		ui.btnDelete.Hide()
	} else {
		ui.btnExtract.Show()
		ui.btnExtractAll.Show()
		ui.fileSaveRow.Show()
		ui.btnDelete.Show()
	}
	ui.fileNav.Refresh()
}

// onMain schedules fn on the Fyne main runtime (safe from worker goroutines).
func (ui *App) onMain(fn func()) {
	fyne.Do(fn)
}

func (ui *App) setStatus(msg string) {
	ui.status.SetText(msg)
}

func (ui *App) setBusyChrome(busy bool) {
	ui.setBusyChromeForOperation(busy, busy)
}

func (ui *App) setBusyChromeForOperation(busy, cancellable bool) {
	if busy {
		ui.progress.Show()
		ui.progress.SetValue(0)
		ui.busyPip.Show()
		if cancellable {
			ui.btnCancel.Show()
			ui.btnCancel.Enable()
		} else {
			ui.btnCancel.Hide()
			ui.btnCancel.Disable()
		}
		ui.btnCreate.Disable()
		ui.btnOpen.Disable()
		ui.btnHide.Disable()
		ui.btnAdd.Disable()
		ui.btnAddFolder.Disable()
		ui.btnExtract.Disable()
		ui.btnExtractAll.Disable()
		ui.btnDelete.Disable()
		ui.btnPassword.Disable()
		ui.btnLock.Disable()
		ui.btnCheckUpdates.Disable()
	} else {
		ui.progress.Hide()
		ui.busyPip.Hide()
		ui.btnCancel.Hide()
		ui.btnCheckUpdates.Enable()
		if ui.v != nil && ui.v.IsUnlocked() {
			ui.btnCreate.Enable()
			ui.btnOpen.Enable()
			ui.btnHide.Enable()
			ui.btnAdd.Enable()
			ui.btnAddFolder.Enable()
			ui.btnPassword.Enable()
			ui.btnLock.Enable()
			ui.setFileActionAvailability()
		} else {
			ui.setLockedUI()
		}
	}
	if ui.root != nil {
		ui.root.Refresh()
	}
}

func (ui *App) setFileActionAvailability() {
	if ui.jobs.Busy() || ui.v == nil || !ui.v.IsUnlocked() || len(ui.files) == 0 {
		ui.btnExtract.Disable()
		ui.btnExtractAll.Disable()
		ui.btnDelete.Disable()
		return
	}
	ui.btnExtractAll.Enable()
	if ui.selected == "" {
		ui.btnExtract.Disable()
		ui.btnDelete.Disable()
		return
	}
	ui.btnExtract.Enable()
	ui.btnDelete.Enable()
}

// runAsync starts work off the UI thread with status + busy chrome.
func (ui *App) runAsync(op, pendingMsg string, work func() error, onOK func()) {
	ui.runAsyncWithOptionsAndCancel(op, pendingMsg, func(context.Context, func(vault.OperationUpdate)) error {
		return work()
	}, false, onOK)
}

func (ui *App) cancelOperation() {
	if ui.cancelJob != nil {
		ui.cancelJob()
		ui.btnCancel.Disable()
		ui.setStatus("Cancelling… finishing the current safe write step.")
	}
}

func (ui *App) runAsyncWithOptions(op, pendingMsg string, work func(context.Context, func(vault.OperationUpdate)) error, onOK func()) {
	ui.runAsyncWithOptionsAndCancel(op, pendingMsg, work, true, onOK)
}

func (ui *App) runAsyncWithOptionsAndCancel(op, pendingMsg string, work func(context.Context, func(vault.OperationUpdate)) error, cancellable bool, onOK func()) {
	if err := ui.jobs.Begin(op, pendingMsg); err != nil {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ui.cancelJob = cancel
	ui.setBusyChromeForOperation(true, cancellable)
	ui.setStatus(StatusLine(true, op, pendingMsg))

	done := make(chan struct{})
	ui.jobDone = done
	go func() {
		var progressMu sync.Mutex
		lastReport := time.Time{}
		lastPhase := ""
		report := func(update vault.OperationUpdate) {
			progressMu.Lock()
			if update.Phase == lastPhase && time.Since(lastReport) < 100*time.Millisecond && update.Total > 0 && update.Done < update.Total {
				progressMu.Unlock()
				return
			}
			lastPhase = update.Phase
			lastReport = time.Now()
			progressMu.Unlock()
			ui.onMain(func() {
				if update.Total > 0 {
					fraction := float64(update.Done) / float64(update.Total)
					if fraction > 1 {
						fraction = 1
					}
					ui.progress.SetValue(fraction)
				} else {
					ui.progress.SetValue(0)
				}
				msg := update.Phase
				if update.Total > 0 {
					msg = fmt.Sprintf("%s · %d%%", msg, int(float64(update.Done)*100/float64(update.Total)))
				}
				ui.setStatus(StatusLine(true, op, msg))
			})
		}
		err := work(ctx, report)
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		ui.onMain(func() {
			defer close(done)
			cancel()
			ui.cancelJob = nil
			if err != nil {
				if errors.Is(err, context.Canceled) {
					ui.jobs.Finish("Cancelled")
					ui.setBusyChrome(false)
					if op == "Download update" {
						ui.setStatus("Update download cancelled; the incomplete file was discarded.")
					} else if op == "Check for updates" {
						ui.setStatus("Update check cancelled; no update was downloaded.")
					} else {
						ui.setStatus("Cancelled — completed file outputs remain; the vault file is unchanged.")
					}
					return
				}
				ui.jobs.Finish("Failed: " + err.Error())
				ui.setBusyChrome(false)
				ui.setStatus(ui.jobs.messageOr("Failed"))
				dialog.ShowError(err, ui.win)
				return
			}
			if onOK != nil {
				onOK()
			}
			if ui.jobs.Busy() {
				ui.jobs.Finish("Done")
			}
			ui.setBusyChrome(false)
		})
	}()
}

func (j *JobState) messageOr(fallback string) string {
	_, _, m := j.Snapshot()
	if m == "" {
		return fallback
	}
	return m
}

func (ui *App) setLockedUI() {
	ui.files = nil
	ui.fileSummaries = nil
	ui.searchNames = nil
	ui.visibleFiles = nil
	ui.search.SetText("")
	ui.selected = ""
	ui.fileList.UnselectAll()
	ui.fileList.Refresh()
	ui.syncStage()
	ui.pathLabel.SetText("No vault selected")
	ui.listHeader.SetText("Encrypted files")
	ui.countLabel.SetText("0 items")
	ui.lockBadge.Text = "  ●  LOCKED  "
	ui.lockBadge.Color = colLockRed
	ui.lockBadge.Refresh()
	ui.btnAdd.Disable()
	ui.btnAddFolder.Disable()
	ui.btnExtract.Disable()
	ui.btnExtractAll.Disable()
	ui.btnDelete.Disable()
	ui.btnPassword.Disable()
	ui.btnLock.Disable()
	if !ui.jobs.Busy() {
		ui.btnCreate.Enable()
		ui.btnOpen.Enable()
		ui.btnHide.Enable()
	}
	ui.refreshShortcutGuide()
	ui.updateActionHierarchy(true)
}

func (ui *App) setUnlockedUI(path string) {
	ui.search.SetText("")
	ui.selected = ""
	ui.fileList.UnselectAll()
	vaultName := filepath.Base(path)
	if vaultName == "" || vaultName == "." {
		vaultName = "Vault"
	}
	if ui.v != nil && ui.v.HasCarrier() {
		ui.pathLabel.SetText(fmt.Sprintf("%s  ·  hidden inside picture · %d byte prefix", vaultName, ui.v.CarrierPrefix()))
	} else {
		ui.pathLabel.SetText(vaultName)
	}
	ui.listHeader.SetText("Encrypted files")
	ui.lockBadge.Text = "  ●  UNLOCKED  "
	ui.lockBadge.Color = colUnlockTeal
	ui.lockBadge.Refresh()
	if !ui.jobs.Busy() {
		ui.btnCreate.Enable()
		ui.btnOpen.Enable()
		ui.btnHide.Enable()
		ui.btnAdd.Enable()
		ui.btnAddFolder.Enable()
		ui.btnExtract.Enable()
		ui.btnExtractAll.Enable()
		ui.btnDelete.Enable()
		ui.btnPassword.Enable()
		ui.btnLock.Enable()
	}
	ui.refreshList()
}

func (ui *App) refreshList() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.files = nil
		ui.fileSummaries = nil
		ui.searchNames = nil
		ui.visibleFiles = nil
		ui.fileList.Refresh()
		ui.countLabel.SetText("0 items")
		ui.syncStage()
		ui.updateActionHierarchy(true)
		return
	}
	names, err := ui.v.ListFiles()
	if err != nil {
		ui.setStatus("List failed: " + err.Error())
		return
	}
	ui.files = names
	ui.refreshShortcutGuide()
	ui.updateActionHierarchy(false)
	ui.fileSummaries = make(map[string]vault.FileSummary, len(names))
	for _, name := range names {
		if summary, summaryErr := ui.v.GetFileSummary(name); summaryErr == nil {
			ui.fileSummaries[name] = summary
		}
	}
	ui.searchNames = nil
	if ui.selected != "" {
		found := false
		for _, name := range names {
			if name == ui.selected {
				found = true
				break
			}
		}
		if !found {
			ui.selected = ""
			ui.fileList.UnselectAll()
		}
	}
	ui.filterFiles()
	n := len(names)
	if ui.jobs.Busy() {
		return
	}
	if n == 0 {
		ui.setStatus("Vault ready — add a file or folder to get started.")
	} else {
		count := fmt.Sprintf("%d files", n)
		if n == 1 {
			count = "1 file"
		}
		ui.setStatus(fmt.Sprintf("Vault open — %s. Select one to save or delete it; use Save all files for everything.", count))
	}
}

func (ui *App) updateActionHierarchy(locked bool) {
	createImportance := widget.MediumImportance
	addImportance := widget.MediumImportance
	if locked {
		createImportance = widget.HighImportance
	} else if len(ui.files) > 0 {
		addImportance = widget.HighImportance
	}
	setButtonImportance(ui.btnCreate, createImportance)
	setButtonImportance(ui.btnOpen, widget.MediumImportance)
	setButtonImportance(ui.btnHide, widget.MediumImportance)
	setButtonImportance(ui.btnAdd, addImportance)
	setButtonImportance(ui.btnAddFolder, widget.MediumImportance)
	setButtonImportance(ui.btnLock, widget.MediumImportance)
	// The action rail owns these controls as a group. Refreshing the parent
	// keeps child renderers in sync when a vault transition updates importance.
	if ui.compactVaultActions != nil {
		ui.compactVaultActions.Refresh()
	}
}

func setButtonImportance(button *widget.Button, importance widget.Importance) {
	// File-list refreshes should not repaint controls whose visual priority is unchanged.
	if button.Importance == importance {
		return
	}
	button.Importance = importance
	button.Refresh()
}

func (ui *App) filterFiles() {
	query := strings.TrimSpace(ui.search.Text)
	ui.searchNames, ui.visibleFiles = filterFileNames(ui.files, ui.searchNames, query, ui.visibleFiles)
	ui.selected = ""
	ui.fileList.UnselectAll()
	ui.fileList.Refresh()
	if query != "" {
		ui.countLabel.SetText(fmt.Sprintf("%d of %d items", len(ui.visibleFiles), len(ui.files)))
	} else if len(ui.files) == 1 {
		ui.countLabel.SetText("1 item")
	} else {
		ui.countLabel.SetText(fmt.Sprintf("%d items", len(ui.files)))
	}
	ui.syncStage()
	ui.setFileActionAvailability()
}

func (ui *App) chooseOutputPath(title, confirm, initialName, initialFolder string, normalize func(string) string, onSave func(string)) {
	name := widget.NewEntry()
	name.SetText(initialName)
	name.SetPlaceHolder("File name")
	name.Validator = validateOutputName

	folder := widget.NewEntry()
	folder.SetText(initialFolder)
	folder.SetPlaceHolder("Choose a folder")
	folder.Validator = validateOutputFolder
	browse := widget.NewButtonWithIcon("Browse…", theme.FolderOpenIcon(), func() {
		showLocalFolder("Save folder", folder.Text, func(chosenPath string) {
			folder.SetText(chosenPath)
		}, ui.win)
	})
	items := []*widget.FormItem{
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Save folder", folder),
		widget.NewFormItem("", browse),
	}
	form := dialog.NewForm(title, confirm, "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		if err := validateOutputName(name.Text); err != nil {
			dialog.ShowError(err, ui.win)
			return
		}
		if err := validateOutputFolder(folder.Text); err != nil {
			dialog.ShowError(err, ui.win)
			return
		}
		path := normalize(filepath.Join(folder.Text, name.Text))
		if err := ensureDestinationAbsent(path); err != nil {
			dialog.ShowError(err, ui.win)
			return
		}
		onSave(path)
	}, ui.win)
	form.Resize(fyne.NewSize(640, 260))
	form.Show()
}

func (ui *App) onCreate() {
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	folder, err := os.UserHomeDir()
	if err != nil {
		folder = "."
	}
	ui.chooseOutputPath("Create vault", "Create", "New vault", folder, PrepareCreatePath, func(path string) {
		ui.promptNewVaultProfile("New vault password hardening", func(profile string) {
			ui.promptPassword("Create vault password", true, func(pw string) {
				if pw == "" {
					ui.setStatus("Create cancelled — empty password.")
					return
				}
				ui.confirmPasswordPolicy(pw, func() {
					var created *vault.Vault
					ui.runAsync("Create vault", "Deriving key & writing V6 container…", func() error {
						v := vault.New(path)
						if err := v.Create(pw, profile); err != nil {
							return err
						}
						created = v
						return nil
					}, func() {
						if ui.v != nil {
							ui.v.Lock()
						}
						ui.v = created
						ui.setUnlockedUI(path)
						ui.jobs.Finish("Vault created: " + filepath.Base(path))
						ui.setStatus("Vault created: " + filepath.Base(path))
					})
				})
			})
		})
	})
}

func (ui *App) promptNewVaultProfile(title string, done func(string)) (*dialog.ConfirmDialog, *widget.Select) {
	standard := kdfProfileOption("standard")
	hardened := kdfProfileOption("hardened")
	choice := widget.NewSelect([]string{standard, hardened}, nil)
	choice.SetSelected(standard)
	details := wrappedLabel(kdfProfileExplanation("standard"))
	choice.OnChanged = func(option string) {
		details.SetText(kdfProfileExplanation(kdfProfileFromOption(option)))
	}
	content := container.NewVBox(
		wrappedLabel("A copy of the vault allows offline password guesses. A long, unique passphrase matters most."),
		container.NewGridWithColumns(2, widget.NewLabel("Key derivation"), choice),
		details,
	)
	d := dialog.NewCustomConfirm(title, "Continue", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		profile := kdfProfileFromOption(choice.Selected)
		if profile == "" {
			ui.setStatus("Choose a password-hardening profile to continue.")
			return
		}
		done(profile)
	}, ui.win)
	d.Resize(fyne.NewSize(560, 310))
	d.Show()
	return d, choice
}

func (ui *App) confirmPasswordPolicy(password string, proceed func()) *dialog.ConfirmDialog {
	warning := vault.PasswordPolicyError(password)
	if warning == "" {
		proceed()
		return nil
	}
	content := wrappedLabel(warning + " This check catches a few common patterns but cannot estimate password strength. Anyone holding a vault copy can test guesses offline; use a long, unique passphrase.")
	d := dialog.NewCustomConfirm("Password strength warning", "Use anyway", "Cancel", content, func(ok bool) {
		if ok {
			proceed()
		}
	}, ui.win)
	d.Resize(fyne.NewSize(520, 230))
	d.Show()
	return d
}

func (ui *App) onOpen() {
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	showLocalFile("Open a vault", func(path string) {
		if path == "" {
			return
		}
		ui.promptPassword("Unlock vault password", false, func(pw string) {
			if pw == "" {
				ui.setStatus("Open cancelled — empty password.")
				return
			}
			var unlocked *vault.Vault
			ui.runAsync("Unlock", "Deriving key & decrypting metadata…", func() error {
				v := vault.New(path)
				if err := v.Unlock(pw); err != nil {
					return fmt.Errorf("unlock failed: wrong password, corrupted vault, or not a vault — you can open a .pulsevault or a picture/video with a hidden vault")
				}
				unlocked = v
				return nil
			}, func() {
				if ui.v != nil {
					ui.v.Lock()
				}
				ui.v = unlocked
				ui.setUnlockedUI(path)
				msg := fmt.Sprintf("Unlocked %s", filepath.Base(path))
				if unlocked.HasCarrier() {
					msg = fmt.Sprintf("Unlocked %s · Hidden inside picture · %d byte prefix", filepath.Base(path), unlocked.CarrierPrefix())
				}
				ui.jobs.Finish(msg)
				ui.setStatus(msg)
			})
		})
	}, ui.win)
}

func (ui *App) onHide() {
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	showLocalFile("Choose a picture", func(carrier string) {
		if carrier == "" {
			return
		}
		msg := widget.NewLabel(
			"Embed in this picture replaces it atomically with picture+vault. The file still opens as a picture.\n\n" +
				"Save a copy writes a new file and leaves the original unchanged.",
		)
		msg.Wrapping = fyne.TextWrapWord
		var chooser dialog.Dialog
		embedBtn := widget.NewButton("Embed in this picture", func() {
			if chooser != nil {
				chooser.Hide()
			}
			dialog.ShowConfirm("Replace the selected media file?", "The original file will be atomically replaced by a copy with the encrypted vault appended. Choose Save a copy to keep the original media file.", func(ok bool) {
				if ok {
					ui.hideWithDest(carrier, carrier)
				}
			}, ui.win)
		})
		embedBtn.Importance = widget.HighImportance
		copyBtn := widget.NewButton("Save a copy", func() {
			if chooser != nil {
				chooser.Hide()
			}
			ui.hideSaveCopy(carrier)
		})
		chooser = dialog.NewCustom("Hide in picture", "Cancel", container.NewVBox(msg, embedBtn, copyBtn), ui.win)
		chooser.Resize(fyne.NewSize(480, 260))
		chooser.Show()
	}, ui.win)
}

func (ui *App) hideSaveCopy(carrier string) {
	carrierBase := filepath.Base(carrier)
	carrierExt := filepath.Ext(carrierBase)
	defaultName := strings.TrimSuffix(carrierBase, carrierExt) + " (copy)" + carrierExt
	ui.chooseOutputPath("Save hidden vault copy", "Save copy", defaultName, filepath.Dir(carrier), func(path string) string {
		return PrepareHidePath(path, carrier)
	}, func(path string) {
		ui.hideWithDest(path, carrier)
	})
}

func (ui *App) hideWithDest(dest, carrier string) {
	ui.promptNewVaultProfile("Hidden vault password hardening", func(profile string) {
		ui.promptPassword("Hide-in-picture password", true, func(pw string) {
			if pw == "" {
				ui.setStatus("Hide cancelled — empty password.")
				return
			}
			ui.confirmPasswordPolicy(pw, func() {
				var created *vault.Vault
				ui.runAsync("Hide in picture", "Embedding vault after the picture…", func() error {
					v := vault.New(dest)
					if err := v.CreateWithCarrier(pw, profile, carrier); err != nil {
						return err
					}
					created = v
					return nil
				}, func() {
					if ui.v != nil {
						ui.v.Lock()
					}
					ui.v = created
					ui.setUnlockedUI(dest)
					base := filepath.Base(dest)
					ui.jobs.Finish("Hidden in picture: " + base)
					ui.setStatus("Vault hidden in " + base + " — the file still opens as a picture.")
				})
			})
		})
	})
}

func (ui *App) onAdd() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.setStatus("Unlock a vault first.")
		return
	}
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	showLocalFile("Add a file", func(path string) {
		if path == "" {
			return
		}
		ui.startAdd([]string{path}, filepath.Base(path))
	}, ui.win)
}

func (ui *App) onAddFolder() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.setStatus("Unlock a vault first.")
		return
	}
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	showLocalFolder("Add a folder", "", func(chosenPath string) {
		ui.startAdd([]string{chosenPath}, filepath.Base(chosenPath))
	}, ui.win)
}

func (ui *App) startAdd(paths []string, label string) {
	if len(paths) == 0 {
		return
	}
	space, err := ui.v.EstimateAddTemporaryBytes(paths)
	if err != nil {
		dialog.ShowError(err, ui.win)
		return
	}
	pending := fmt.Sprintf("Encrypting %s… temporary space may peak near %s", label, formatBytes(space))
	ui.runAsyncWithOptions("Add files", pending, func(ctx context.Context, report func(vault.OperationUpdate)) error {
		return ui.v.AddPathsWithOptions(paths, false, vault.OperationOptions{Context: ctx, Progress: report})
	}, func() {
		ui.refreshList()
		msg := "Added selected files"
		if len(paths) == 1 {
			msg = "Added " + label
		}
		ui.jobs.Finish(msg)
		ui.setStatus(msg)
	})
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		value /= 1024
		if value < 1024 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}

func (ui *App) onExtract() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.setStatus("Unlock a vault first.")
		return
	}
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	if len(ui.files) == 0 {
		ui.setStatus("Vault is empty.")
		return
	}
	name := ui.selected
	if name == "" {
		ui.setStatus("Select a file first. Choose Save all files to save everything.")
		return
	}
	nameEntry := widget.NewEntry()
	nameEntry.SetText(name)
	nameEntry.SetPlaceHolder("Filename in vault")
	form := dialog.NewForm(
		"Save a copy",
		"Choose folder…",
		"Cancel",
		[]*widget.FormItem{widget.NewFormItem("Name", nameEntry)},
		func(ok bool) {
			if !ok {
				return
			}
			n := strings.TrimSpace(nameEntry.Text)
			if n == "" {
				ui.setStatus("No filename given.")
				return
			}
			showLocalFolder("Save file to folder", "", func(chosenPath string) {
				ui.startExtract(n, chosenPath, false)
			}, ui.win)
		},
		ui.win,
	)
	form.Resize(fyne.NewSize(440, 180))
	form.Show()
}

func (ui *App) onExtractAll() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.setStatus("Unlock a vault first.")
		return
	}
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	if len(ui.files) == 0 {
		ui.setStatus("Vault is empty.")
		return
	}
	showLocalFolder("Save all files to folder", "", func(chosenPath string) {
		ui.startExtractAll(chosenPath, false)
	}, ui.win)
}

func (ui *App) startExtractAll(outputDir string, overwrite bool) {
	conflict := false
	ui.runAsyncWithOptions("Save all files", fmt.Sprintf("Saving %d files…", len(ui.files)), func(ctx context.Context, report func(vault.OperationUpdate)) error {
		_, err := ui.v.ExtractAllWithOptions(outputDir, overwrite, vault.OperationOptions{Context: ctx, Progress: report})
		if !overwrite && extractAlreadyExists(err) {
			conflict = true
			return nil
		}
		return err
	}, func() {
		if conflict {
			dialog.ShowConfirm("Overwrite existing files?", "One or more files already exist in this folder. Overwrite them?", func(yes bool) {
				if yes {
					ui.startExtractAll(outputDir, true)
				} else {
					ui.setStatus("Save all cancelled — existing files were kept.")
				}
			}, ui.win)
			return
		}
		ui.jobs.Finish("Saved all files to " + outputDir)
		ui.setStatus("Saved all files to " + outputDir)
	})
}

func (ui *App) startExtract(n, outDir string, overwrite bool) {
	var outPath string
	existsConflict := false
	ui.runAsyncWithOptions("Save a copy", "Saving "+n+"…", func(ctx context.Context, report func(vault.OperationUpdate)) error {
		p, err := ui.v.ExtractFileWithOptions(n, outDir, overwrite, vault.OperationOptions{Context: ctx, Progress: report})
		if err != nil {
			if !overwrite && extractAlreadyExists(err) {
				existsConflict = true
				return nil
			}
			return err
		}
		outPath = p
		return nil
	}, func() {
		if existsConflict {
			dialog.ShowConfirm(
				"Overwrite existing file?",
				fmt.Sprintf("%q already exists in the output folder. Overwrite?", n),
				func(yes bool) {
					if !yes {
						ui.setStatus("Save cancelled — that file already exists.")
						return
					}
					ui.startExtract(n, outDir, true)
				},
				ui.win,
			)
			return
		}
		ui.jobs.Finish("Saved a copy to " + outPath)
		ui.setStatus("Saved a copy to " + outPath)
	})
}

func extractAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already exists in the output folder")
}

func (ui *App) onDelete() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.setStatus("Unlock a vault first.")
		return
	}
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	if len(ui.files) == 0 {
		ui.setStatus("Vault is empty.")
		return
	}
	name := ui.selected
	if name == "" {
		ui.setStatus("Select a file first to delete it.")
		return
	}
	space, err := ui.v.EstimateDeleteTemporaryBytes()
	if err != nil {
		dialog.ShowError(err, ui.win)
		return
	}
	dialog.ShowConfirm(
		"Delete file",
		fmt.Sprintf("Permanently delete %q from the vault? This cannot be undone. Rebuilding the vault may need about %s of temporary space.", name, formatBytes(space)),
		func(ok bool) {
			if !ok {
				return
			}
			ui.runAsyncWithOptions("Delete", "Removing "+name+"…", func(ctx context.Context, report func(vault.OperationUpdate)) error {
				return ui.v.DeleteFileWithOptions(name, vault.OperationOptions{Context: ctx, Progress: report})
			}, func() {
				if ui.selected == name {
					ui.selected = ""
				}
				ui.refreshList()
				ui.jobs.Finish("Deleted " + name)
				ui.setStatus("Deleted " + name)
			})
		},
		ui.win,
	)
}

func (ui *App) onPassword() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.setStatus("Unlock a vault first.")
		return
	}
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	keepCurrent := keepCurrentKDFOption(ui.v.Profile)
	profile := widget.NewSelect([]string{keepCurrent, kdfProfileOption("standard"), kdfProfileOption("hardened")}, nil)
	profile.SetSelected(keepCurrent)
	profileInfo := wrappedLabel(kdfProfileExplanation(""))
	profile.OnChanged = func(option string) {
		if option == keepCurrent {
			profileInfo.SetText(kdfProfileExplanation(""))
			return
		}
		profileInfo.SetText(kdfProfileExplanation(kdfProfileFromOption(option)))
	}
	old := widget.NewPasswordEntry()
	old.SetPlaceHolder("Current password")
	neu := widget.NewPasswordEntry()
	neu.SetPlaceHolder("New password")
	conf := widget.NewPasswordEntry()
	conf.SetPlaceHolder("Confirm new password")
	items := []*widget.FormItem{
		widget.NewFormItem("Current", old),
		widget.NewFormItem("New", neu),
		widget.NewFormItem("Confirm", conf),
		widget.NewFormItem("KDF profile", profile),
		widget.NewFormItem("Profile info", profileInfo),
	}
	d := dialog.NewForm("Change password", "OK", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		if neu.Text != conf.Text {
			dialog.ShowError(fmt.Errorf("passwords do not match"), ui.win)
			return
		}
		if old.Text == "" || neu.Text == "" {
			dialog.ShowError(fmt.Errorf("password cannot be empty"), ui.win)
			return
		}
		profileName := ""
		if profile.Selected != keepCurrent {
			profileName = kdfProfileFromOption(profile.Selected)
			if profileName == "" {
				dialog.ShowError(fmt.Errorf("choose a supported KDF profile"), ui.win)
				return
			}
		}
		ui.confirmPasswordPolicy(neu.Text, func() {
			space, err := ui.v.EstimatePasswordRotationTemporaryBytes()
			if err != nil {
				dialog.ShowError(err, ui.win)
				return
			}
			pending := fmt.Sprintf("Re-encrypting vault… temporary space may peak near %s", formatBytes(space))
			ui.runAsyncWithOptions("Change password", pending, func(ctx context.Context, report func(vault.OperationUpdate)) error {
				return ui.v.ChangePasswordWithProfileAndOptions(old.Text, neu.Text, profileName, vault.OperationOptions{Context: ctx, Progress: report})
			}, func() {
				ui.jobs.Finish("Password changed")
				ui.setStatus("Password changed.")
			})
		})
	}, ui.win)
	d.Resize(fyne.NewSize(520, 390))
	d.Show()
}

func (ui *App) onLock() {
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	if ui.v != nil {
		ui.v.Lock()
		ui.v = nil
	}
	ui.setLockedUI()
	ui.setStatus("Vault locked.")
}

func (ui *App) promptPassword(title string, confirm bool, done func(string)) {
	pw1 := widget.NewPasswordEntry()
	pw1.SetPlaceHolder("Password")
	passwordLabel := "Password"
	if confirm {
		passwordLabel = "Password (cannot be reset)"
	}
	items := []*widget.FormItem{widget.NewFormItem(passwordLabel, pw1)}
	var pw2 *widget.Entry
	if confirm {
		pw2 = widget.NewPasswordEntry()
		pw2.SetPlaceHolder("Confirm password")
		items = append(items, widget.NewFormItem("Confirm password", pw2))
	}
	d := dialog.NewForm(title, "OK", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		if confirm && pw2 != nil && pw1.Text != pw2.Text {
			dialog.ShowError(fmt.Errorf("passwords do not match"), ui.win)
			return
		}
		done(pw1.Text)
	}, ui.win)
	d.Resize(fyne.NewSize(400, 200))
	d.Show()
}
