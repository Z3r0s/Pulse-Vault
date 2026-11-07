// Package ui is the fully custom Pulse-Vault desktop shell.
// Long vault ops run off the UI thread; results apply via fyne.Do (main runtime).
package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/version"
)

const AppTitle = "Pulse-Vault"

// App is the multi-pane vault desktop UI.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	v       *vault.Vault
	jobs    JobState

	pathLabel         *widget.Label
	status            *widget.Label
	progress          *widget.ProgressBar
	cancelJob         context.CancelFunc
	fileList          *widget.List
	files             []string
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

	btnCreate     *widget.Button
	btnOpen       *widget.Button
	btnHide       *widget.Button
	btnAdd        *widget.Button
	btnAddFolder  *widget.Button
	btnExtract    *widget.Button
	btnExtractAll *widget.Button
	btnCancel     *widget.Button
	btnDelete     *widget.Button
	btnPassword   *widget.Button
	btnLock       *widget.Button
}

// Run launches the desktop window (blocks until closed).
func Run(a fyne.App) {
	ui := &App{fyneApp: a}
	mark := brandResource()
	a.SetIcon(mark)
	ui.win = a.NewWindow(AppTitle + "  ·  " + version.Version)
	ui.win.SetIcon(mark)
	ui.win.SetPadded(false)
	ui.win.Resize(fyne.NewSize(1120, 700))
	ui.win.SetFixedSize(false)
	ui.win.SetMaster()
	ui.build()
	ui.bindShortcuts()
	ui.win.SetOnDropped(ui.onDropped)
	ui.setLockedUI()
	ui.win.ShowAndRun()
}

func (ui *App) build() {
	ui.brandMark = newBrandImage(52)

	ui.brandTitle = canvas.NewText("PULSE-VAULT", colFg)
	ui.brandTitle.TextStyle = fyne.TextStyle{Bold: true}
	ui.brandTitle.TextSize = 22

	ui.brandSub = canvas.NewText("DNSPulse  ·  dnspulse.org  ·  offline", colMuted)
	ui.brandSub.TextSize = 12

	ui.versionChip = canvas.NewText("  v"+version.Version+"  ", colPrimary)
	ui.versionChip.TextSize = 11
	ui.versionChip.TextStyle = fyne.TextStyle{Bold: true}

	ui.lockBadge = canvas.NewText("  ●  LOCKED  ", colLockRed)
	ui.lockBadge.TextStyle = fyne.TextStyle{Bold: true}
	ui.lockBadge.TextSize = 12

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

	ui.listHeader = widget.NewLabelWithStyle("Secure files", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
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
			row := obj.(*fyne.Container)
			for _, child := range row.Objects {
				if lab, ok := child.(*widget.Label); ok {
					lab.SetText(ui.visibleFiles[id])
					return
				}
			}
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
	ui.heroTitle.TextSize = 24
	ui.heroTitle.Alignment = fyne.TextAlignCenter
	ui.heroBody = widget.NewLabel(
		"Create or open a vault. Adding files copies them; originals stay in place.",
	)
	ui.heroBody.Alignment = fyne.TextAlignCenter
	ui.heroBody.Wrapping = fyne.TextWrapWord
	ui.heroLogo = newBrandImage(108)
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
	heroGlow := canvas.NewRadialGradient(
		color.NRGBA{R: 0x00, G: 0xe5, B: 0xc0, A: 0x28},
		color.NRGBA{A: 0x00},
	)
	ui.heroCard = container.NewStack(
		heroBg,
		heroGlow,
		container.NewPadded(heroInner),
	)

	listPane := container.NewStack(ui.heroCard, ui.fileList, container.NewCenter(ui.noSearchResults))
	ui.fileList.Hide()

	ui.btnCreate = widget.NewButtonWithIcon("  Create vault", theme.DocumentCreateIcon(), ui.onCreate)
	ui.btnCreate.Importance = widget.HighImportance
	ui.btnOpen = widget.NewButtonWithIcon("  Open vault", theme.FolderOpenIcon(), ui.onOpen)
	ui.btnHide = widget.NewButtonWithIcon("  Hide in picture", theme.MediaPhotoIcon(), ui.onHide)
	ui.btnAdd = widget.NewButtonWithIcon("  Add file", theme.ContentAddIcon(), ui.onAdd)
	ui.btnAddFolder = widget.NewButtonWithIcon("  Add folder", theme.FolderOpenIcon(), ui.onAddFolder)
	ui.btnExtract = widget.NewButtonWithIcon("  Save a copy", theme.DownloadIcon(), ui.onExtract)
	ui.btnExtractAll = widget.NewButton("  Save all files", ui.onExtractAll)
	ui.btnDelete = widget.NewButtonWithIcon("  Delete", theme.DeleteIcon(), ui.onDelete)
	ui.btnPassword = widget.NewButtonWithIcon("  Change password", theme.LoginIcon(), ui.onPassword)
	ui.btnLock = widget.NewButtonWithIcon("  Lock vault", theme.LogoutIcon(), ui.onLock)
	ui.btnLock.Importance = widget.DangerImportance

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
	privacy.TextSize = 11
	shortcuts := widget.NewLabel("Ctrl+N  New vault\nCtrl+O  Open vault\nCtrl+F  Find file\nCtrl+L  Lock vault")
	shortcuts.Importance = widget.LowImportance
	ui.fileNav = container.NewVBox(
		widget.NewSeparator(),
		ui.sectionFiles,
		ui.btnAdd,
		ui.btnAddFolder,
		ui.btnExtract,
		ui.btnExtractAll,
		ui.btnDelete,
		widget.NewSeparator(),
		ui.btnPassword,
		ui.btnLock,
	)
	ui.fileNav.Hide()
	nav := container.NewVBox(
		ui.sectionVault,
		ui.btnCreate,
		ui.btnOpen,
		ui.btnHide,
		ui.fileNav,
		shortcuts,
		container.NewPadded(privacy),
	)
	railAccent := canvas.NewRectangle(colPrimary)
	railAccent.SetMinSize(fyne.NewSize(3, 1))
	railBg := canvas.NewRectangle(colSidebar)
	navFloor := canvas.NewRectangle(color.Transparent)
	navFloor.SetMinSize(fyne.NewSize(navTargetWidth, 1))
	navScroll := container.NewVScroll(container.NewPadded(nav))
	ui.navRail = container.NewStack(
		railBg,
		navFloor,
		container.NewBorder(nil, nil, railAccent, nil, navScroll),
	)

	headerRow := container.NewBorder(nil, nil, ui.listHeader, ui.countLabel)
	stageHeader := container.NewVBox(
		headerRow,
		ui.pathLabel,
		ui.search,
		widget.NewSeparator(),
	)
	stageBg := canvas.NewRectangle(colPanel)
	stageInner := container.New(&stageContentLayout{}, container.NewPadded(stageHeader), listPane)
	ui.mainStage = container.NewStack(stageBg, stageInner)

	statusBg := canvas.NewRectangle(colTopBar)
	statusRow := container.NewBorder(nil, nil, ui.busyPip, ui.btnCancel, ui.status)
	statusInner := container.NewVBox(ui.progress, statusRow)
	statusDock := container.NewStack(statusBg, container.NewPadded(statusInner))

	ui.split = container.NewHSplit(ui.navRail, ui.mainStage)
	ui.split.Offset = 0.24
	fitted := container.New(&splitFitLayout{}, ui.split)
	ui.root = container.NewBorder(ui.topBar, statusDock, nil, nil, fitted)
	ui.win.SetContent(ui.root)
}

func newFileListRow() fyne.CanvasObject {
	icon := widget.NewIcon(theme.FileIcon())
	name := widget.NewLabel("filename.ext")
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.Wrapping = fyne.TextWrapOff
	name.Truncation = fyne.TextTruncateEllipsis
	tag := canvas.NewText("ENCRYPTED", colPrimary)
	tag.TextSize = 10
	return container.NewBorder(nil, nil, icon, tag, name)
}

func (ui *App) bindShortcuts() {
	c := ui.win.Canvas()
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) { ui.onCreate() })
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) { ui.onOpen() })
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyL, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) { ui.onLock() })
	c.AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyF, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) {
		if ui.search.Visible() && !ui.jobs.Busy() {
			c.Focus(ui.search)
		}
	})
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
		ui.fileList.Show()
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
		ui.btnDelete.Hide()
	} else {
		ui.btnExtract.Show()
		ui.btnExtractAll.Show()
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
	} else {
		ui.progress.Hide()
		ui.busyPip.Hide()
		ui.btnCancel.Hide()
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
			cancel()
			ui.cancelJob = nil
			if err != nil {
				if errors.Is(err, context.Canceled) {
					ui.jobs.Finish("Cancelled")
					ui.setBusyChrome(false)
					ui.setStatus("Cancelled — completed file outputs remain; the vault file is unchanged.")
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
	ui.visibleFiles = nil
	ui.search.SetText("")
	ui.selected = ""
	ui.fileList.UnselectAll()
	ui.fileList.Refresh()
	ui.syncStage()
	ui.pathLabel.SetText("No vault selected")
	ui.listHeader.SetText("Secure files")
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
}

func (ui *App) setUnlockedUI(path string) {
	ui.search.SetText("")
	ui.selected = ""
	ui.fileList.UnselectAll()
	if ui.v != nil && ui.v.HasCarrier() {
		ui.pathLabel.SetText(fmt.Sprintf("%s  ·  hidden inside picture · %d byte prefix", path, ui.v.CarrierPrefix()))
	} else {
		ui.pathLabel.SetText(path)
	}
	ui.listHeader.SetText("Secure files — unlocked")
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
		ui.visibleFiles = nil
		ui.fileList.Refresh()
		ui.countLabel.SetText("0 items")
		ui.syncStage()
		return
	}
	names, err := ui.v.ListFiles()
	if err != nil {
		ui.setStatus("List failed: " + err.Error())
		return
	}
	ui.files = names
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

func (ui *App) filterFiles() {
	query := strings.ToLower(strings.TrimSpace(ui.search.Text))
	ui.visibleFiles = make([]string, 0, len(ui.files))
	for _, name := range ui.files {
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			ui.visibleFiles = append(ui.visibleFiles, name)
		}
	}
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

func (ui *App) onCreate() {
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	dialog.ShowFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil {
			ui.setStatus("Create cancelled: " + err.Error())
			return
		}
		if uc == nil {
			return
		}
		path := uc.URI().Path()
		_ = uc.Close()
		if path == "" {
			return
		}
		final, remove := PrepareCreatePath(path)
		for _, p := range remove {
			_ = os.Remove(p)
		}
		path = final
		ui.promptPassword("Create vault password", true, func(pw string) {
			if pw == "" {
				ui.setStatus("Create cancelled — empty password.")
				return
			}
			if warn := vault.PasswordPolicyError(pw); warn != "" {
				dialog.ShowInformation("Password warning", warn, ui.win)
			}
			var created *vault.Vault
			ui.runAsync("Create vault", "Deriving key & writing V6 container…", func() error {
				v := vault.New(path)
				if err := v.Create(pw, "standard"); err != nil {
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
	}, ui.win)
}

func (ui *App) onOpen() {
	if ui.jobs.Busy() {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
		if err != nil {
			ui.setStatus("Open cancelled: " + err.Error())
			return
		}
		if uc == nil {
			return
		}
		path := uc.URI().Path()
		_ = uc.Close()
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
	dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
		if err != nil {
			ui.setStatus("Hide cancelled: " + err.Error())
			return
		}
		if uc == nil {
			return
		}
		carrier := uc.URI().Path()
		_ = uc.Close()
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
			ui.hideWithDest(carrier, carrier)
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
	dialog.ShowFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil {
			ui.setStatus("Hide cancelled: " + err.Error())
			return
		}
		if uc == nil {
			return
		}
		path := uc.URI().Path()
		_ = uc.Close()
		if path == "" {
			return
		}
		final, remove := PrepareHidePath(path, carrier)
		for _, p := range remove {
			_ = os.Remove(p)
		}
		ui.hideWithDest(final, carrier)
	}, ui.win)
}

func (ui *App) hideWithDest(dest, carrier string) {
	ui.promptPassword("Hide-in-picture password", true, func(pw string) {
		if pw == "" {
			ui.setStatus("Hide cancelled — empty password.")
			return
		}
		if warn := vault.PasswordPolicyError(pw); warn != "" {
			dialog.ShowInformation("Password warning", warn, ui.win)
		}
		var created *vault.Vault
		ui.runAsync("Hide in picture", "Embedding vault after the picture…", func() error {
			v := vault.New(dest)
			if err := v.CreateWithCarrier(pw, "standard", carrier); err != nil {
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
	dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		path := uc.URI().Path()
		_ = uc.Close()
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
	dialog.ShowFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil || folder == nil {
			return
		}
		path := folder.Path()
		if path != "" {
			ui.startAdd([]string{path}, filepath.Base(path))
		}
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
			dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
				if err != nil || lu == nil {
					return
				}
				outDir := lu.Path()
				ui.startExtract(n, outDir, false)
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
	dialog.ShowFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil || folder == nil {
			return
		}
		ui.startExtractAll(folder.Path(), false)
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
		if warn := vault.PasswordPolicyError(neu.Text); warn != "" {
			dialog.ShowInformation("Password warning", warn, ui.win)
		}
		space, err := ui.v.EstimatePasswordRotationTemporaryBytes()
		if err != nil {
			dialog.ShowError(err, ui.win)
			return
		}
		pending := fmt.Sprintf("Re-encrypting vault… temporary space may peak near %s", formatBytes(space))
		ui.runAsyncWithOptions("Change password", pending, func(ctx context.Context, report func(vault.OperationUpdate)) error {
			return ui.v.ChangePasswordWithOptions(old.Text, neu.Text, vault.OperationOptions{Context: ctx, Progress: report})
		}, func() {
			ui.jobs.Finish("Password changed")
			ui.setStatus("Password changed.")
		})
	}, ui.win)
	d.Resize(fyne.NewSize(420, 260))
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
