// Package ui is the fully custom Pulse-Vault desktop shell.
// Long vault ops run off the UI thread; results apply via fyne.Do (main runtime).
package ui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
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
	fyneApp fyne.App
	win     fyne.Window
	v       *vault.Vault
	jobs    JobState

	pathLabel    *widget.Label
	status       *widget.Label
	progress     *widget.ProgressBarInfinite
	fileList     *widget.List
	files        []string
	selected     string
	heroCard     *fyne.Container
	heroTitle    *canvas.Text
	heroBody     *widget.Label
	heroLogo     *canvas.Image
	heroActions  *fyne.Container
	listHeader   *widget.Label
	countLabel   *widget.Label
	lockBadge    *canvas.Text
	lockBadgeBg  *canvas.Rectangle
	brandMark    *canvas.Image
	brandTitle   *canvas.Text
	brandSub     *canvas.Text
	sectionVault *widget.Label
	sectionFiles *widget.Label
	versionChip  *canvas.Text
	accentRule   *canvas.Rectangle
	busyPip      *canvas.Rectangle
	lockPulse    *fyne.Animation
	accentPulse  *fyne.Animation
	heroPulse    *fyne.Animation
	split        *container.Split
	navRail      *fyne.Container
	topBar       *fyne.Container
	mainStage    *fyne.Container
	rootScroll   *container.Scroll

	btnCreate   *widget.Button
	btnOpen     *widget.Button
	btnHide     *widget.Button
	btnAdd      *widget.Button
	btnExtract  *widget.Button
	btnDelete   *widget.Button
	btnPassword *widget.Button
	btnLock     *widget.Button
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
	ui.startLockPulse()
	ui.startAccentPulse()
	ui.startHeroBreath()
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
	ui.pathLabel.Wrapping = fyne.TextWrapWord
	ui.pathLabel.Importance = widget.LowImportance

	ui.status = widget.NewLabel("Ready — create or open a vault.")
	ui.status.Wrapping = fyne.TextWrapWord
	ui.progress = widget.NewProgressBarInfinite()
	ui.progress.Hide()

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
		func() int { return len(ui.files) },
		func() fyne.CanvasObject {
			ico := widget.NewIcon(theme.FileIcon())
			name := widget.NewLabel("filename.ext")
			name.TextStyle = fyne.TextStyle{Bold: true}
			name.Wrapping = fyne.TextWrapOff
			tag := canvas.NewText("ENCRYPTED", colPrimary)
			tag.TextSize = 10
			return container.NewHBox(ico, name, layout.NewSpacer(), tag)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := obj.(*fyne.Container)
			for _, child := range row.Objects {
				if lab, ok := child.(*widget.Label); ok {
					lab.SetText(ui.files[id])
					return
				}
			}
		},
	)
	ui.fileList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(ui.files) {
			ui.selected = ui.files[id]
			if !ui.jobs.Busy() {
				ui.setStatus("Selected · " + ui.selected)
			}
		}
	}

	ui.heroTitle = canvas.NewText("Your vault is locked", colFg)
	ui.heroTitle.TextStyle = fyne.TextStyle{Bold: true}
	ui.heroTitle.TextSize = 24
	ui.heroTitle.Alignment = fyne.TextAlignCenter
	ui.heroBody = widget.NewLabel(
		"New vault, open one, or hide it in a picture.\n" +
			"ChaCha20-Poly1305 + AES-GCM. Files come back bit-exact.",
	)
	ui.heroBody.Alignment = fyne.TextAlignCenter
	ui.heroBody.Wrapping = fyne.TextWrapWord
	ui.heroLogo = newBrandImage(108)
	heroNew := widget.NewButton("New vault", ui.onCreate)
	heroNew.Importance = widget.HighImportance
	heroOpen := widget.NewButton("Open", ui.onOpen)
	heroHide := widget.NewButton("Hide in picture", ui.onHide)
	ui.heroActions = container.NewCenter(container.NewGridWrap(fyne.NewSize(150, 38), heroNew, heroOpen, heroHide))
	heroInner := container.NewVBox(
		container.NewCenter(ui.heroLogo),
		ui.heroTitle,
		ui.heroBody,
		widget.NewSeparator(),
		ui.heroActions,
	)
	heroBg := canvas.NewRectangle(colCard)
	heroBg.CornerRadius = 12
	heroGlow := canvas.NewRadialGradient(
		color.NRGBA{R: 0x00, G: 0xe5, B: 0xc0, A: 0x28},
		color.NRGBA{A: 0x00},
	)
	ui.heroCard = container.NewStack(
		heroBg,
		heroGlow,
		container.NewCenter(container.NewPadded(heroInner)),
	)

	listPane := container.NewStack(ui.heroCard, ui.fileList)
	ui.fileList.Hide()

	ui.btnCreate = widget.NewButtonWithIcon("  New vault", theme.DocumentCreateIcon(), ui.onCreate)
	ui.btnCreate.Importance = widget.HighImportance
	ui.btnOpen = widget.NewButtonWithIcon("  Open vault", theme.FolderOpenIcon(), ui.onOpen)
	ui.btnHide = widget.NewButtonWithIcon("  Hide in picture", theme.MediaPhotoIcon(), ui.onHide)
	ui.btnAdd = widget.NewButtonWithIcon("  Add file", theme.ContentAddIcon(), ui.onAdd)
	ui.btnExtract = widget.NewButtonWithIcon("  Extract", theme.DownloadIcon(), ui.onExtract)
	ui.btnDelete = widget.NewButtonWithIcon("  Delete", theme.DeleteIcon(), ui.onDelete)
	ui.btnPassword = widget.NewButtonWithIcon("  Password", theme.LoginIcon(), ui.onPassword)
	ui.btnLock = widget.NewButtonWithIcon("  Lock", theme.LogoutIcon(), ui.onLock)
	ui.btnLock.Importance = widget.DangerImportance

	accentBar := canvas.NewRectangle(colPrimary)
	accentBar.CornerRadius = 2
	accentBar.SetMinSize(fyne.NewSize(4, 40))
	titleBlock := container.NewVBox(ui.brandTitle, ui.brandSub)
	topLeft := container.NewHBox(
		container.NewPadded(ui.brandMark),
		container.NewPadded(accentBar),
		container.NewPadded(titleBlock),
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
	nav := container.NewVBox(
		ui.sectionVault,
		ui.btnCreate,
		ui.btnOpen,
		ui.btnHide,
		widget.NewSeparator(),
		ui.sectionFiles,
		ui.btnAdd,
		ui.btnExtract,
		ui.btnDelete,
		ui.btnPassword,
		ui.btnLock,
		layout.NewSpacer(),
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
		widget.NewSeparator(),
	)
	stageBg := canvas.NewRectangle(colPanel)
	stageInner := container.NewBorder(
		container.NewPadded(stageHeader),
		nil, nil, nil,
		container.NewPadded(listPane),
	)
	ui.mainStage = container.NewStack(stageBg, stageInner)

	statusBg := canvas.NewRectangle(colTopBar)
	statusRow := container.NewBorder(nil, nil, ui.busyPip, nil, ui.status)
	statusInner := container.NewVBox(ui.progress, statusRow)
	statusDock := container.NewStack(statusBg, container.NewPadded(statusInner))

	ui.split = container.NewHSplit(ui.navRail, ui.mainStage)
	ui.split.Offset = 0.24
	fitted := container.New(&splitFitLayout{}, ui.split)
	root := container.NewBorder(ui.topBar, statusDock, nil, nil, fitted)
	ui.rootScroll = container.NewScroll(root)
	ui.rootScroll.Direction = container.ScrollBoth
	ui.win.SetContent(ui.rootScroll)
}

func (ui *App) startLockPulse() {
	from := colLockRed
	to := color.NRGBA{R: 0xff, G: 0xa0, B: 0xaa, A: 0xff}
	ui.lockPulse = canvas.NewColorRGBAAnimation(from, to, 1600*time.Millisecond, func(c color.Color) {
		if ui.v != nil && ui.v.IsUnlocked() {
			return
		}
		ui.lockBadge.Color = c
		ui.lockBadge.Refresh()
	})
	ui.lockPulse.RepeatCount = fyne.AnimationRepeatForever
	ui.lockPulse.AutoReverse = true
	ui.lockPulse.Start()
}

func (ui *App) startAccentPulse() {
	from := colPrimary
	to := color.NRGBA{R: 0x00, G: 0x7a, B: 0x68, A: 0xff}
	ui.accentPulse = canvas.NewColorRGBAAnimation(from, to, 2200*time.Millisecond, func(c color.Color) {
		if ui.accentRule == nil {
			return
		}
		ui.accentRule.FillColor = c
		ui.accentRule.Refresh()
	})
	ui.accentPulse.RepeatCount = fyne.AnimationRepeatForever
	ui.accentPulse.AutoReverse = true
	ui.accentPulse.Start()
}

func (ui *App) startHeroBreath() {
	small := fyne.NewSize(100, 100)
	large := fyne.NewSize(118, 118)
	ui.heroPulse = canvas.NewSizeAnimation(small, large, 2400*time.Millisecond, func(s fyne.Size) {
		if ui.heroLogo == nil || !ui.heroCard.Visible() {
			return
		}
		ui.heroLogo.SetMinSize(s)
		ui.heroLogo.Refresh()
	})
	ui.heroPulse.RepeatCount = fyne.AnimationRepeatForever
	ui.heroPulse.AutoReverse = true
	ui.heroPulse.Start()
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
}

func (ui *App) onDropped(_ fyne.Position, uris []fyne.URI) {
	if ui.jobs.Busy() || len(uris) == 0 {
		return
	}
	path := uris[0].Path()
	if path == "" {
		return
	}
	if ui.v != nil && ui.v.IsUnlocked() {
		base := filepath.Base(path)
		ui.runAsync("Add file", "Encrypting "+base+"…", func() error {
			return ui.v.AddFile(path, false)
		}, func() {
			ui.refreshList()
			ui.jobs.Finish("Added " + base)
			ui.setStatus("Added " + base)
		})
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
	unlocked := ui.v != nil && ui.v.IsUnlocked()
	if !unlocked {
		ui.heroTitle.Text = "Your vault is locked"
		ui.heroTitle.Refresh()
		ui.heroBody.SetText("New vault, open one, or hide it in a picture.\nChaCha20-Poly1305 + AES-GCM. Files come back bit-exact.")
		ui.heroActions.Show()
		ui.heroCard.Show()
		ui.fileList.Hide()
		return
	}
	ui.heroActions.Hide()
	if len(ui.files) == 0 {
		ui.heroTitle.Text = "Vault is empty"
		ui.heroTitle.Refresh()
		ui.heroBody.SetText("Add a file. It stays encrypted on disk.\nExtract brings every byte back, including pictures.")
		ui.heroCard.Show()
		ui.fileList.Hide()
		return
	}
	ui.heroCard.Hide()
	ui.fileList.Show()
}

// onMain schedules fn on the Fyne main runtime (safe from worker goroutines).
func (ui *App) onMain(fn func()) {
	fyne.Do(fn)
}

func (ui *App) setStatus(msg string) {
	ui.status.SetText(msg)
}

func (ui *App) setBusyChrome(busy bool) {
	if busy {
		ui.progress.Show()
		ui.progress.Start()
		ui.busyPip.Show()
		ui.btnCreate.Disable()
		ui.btnOpen.Disable()
		ui.btnHide.Disable()
		ui.btnAdd.Disable()
		ui.btnExtract.Disable()
		ui.btnDelete.Disable()
		ui.btnPassword.Disable()
		ui.btnLock.Disable()
	} else {
		ui.progress.Stop()
		ui.progress.Hide()
		ui.busyPip.Hide()
		if ui.v != nil && ui.v.IsUnlocked() {
			ui.btnCreate.Enable()
			ui.btnOpen.Enable()
			ui.btnHide.Enable()
			ui.btnAdd.Enable()
			ui.btnExtract.Enable()
			ui.btnDelete.Enable()
			ui.btnPassword.Enable()
			ui.btnLock.Enable()
		} else {
			ui.setLockedUI()
		}
	}
}

// runAsync starts work off the UI thread with status + busy chrome.
func (ui *App) runAsync(op, pendingMsg string, work func() error, onOK func()) {
	if err := ui.jobs.Begin(op, pendingMsg); err != nil {
		ui.setStatus("Busy — wait for the current operation to finish.")
		return
	}
	ui.setBusyChrome(true)
	ui.setStatus(StatusLine(true, op, pendingMsg))

	go func() {
		err := work()
		ui.onMain(func() {
			if err != nil {
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
	ui.selected = ""
	ui.fileList.Refresh()
	ui.syncStage()
	ui.pathLabel.SetText("No vault selected")
	ui.listHeader.SetText("Secure files")
	ui.countLabel.SetText("0 items")
	ui.lockBadge.Text = "  ●  LOCKED  "
	ui.lockBadge.Color = colLockRed
	ui.lockBadge.Refresh()
	if ui.lockPulse != nil {
		ui.lockPulse.Start()
	}
	ui.btnAdd.Disable()
	ui.btnExtract.Disable()
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
	if ui.v != nil && ui.v.HasCarrier() {
		ui.pathLabel.SetText(fmt.Sprintf("%s  ·  hidden inside picture · %d byte prefix", path, ui.v.CarrierPrefix()))
	} else {
		ui.pathLabel.SetText(path)
	}
	ui.listHeader.SetText("Secure files — unlocked")
	ui.lockBadge.Text = "  ●  UNLOCKED  "
	ui.lockBadge.Color = colUnlockTeal
	ui.lockBadge.Refresh()
	if ui.lockPulse != nil {
		ui.lockPulse.Stop()
	}
	if !ui.jobs.Busy() {
		ui.btnCreate.Enable()
		ui.btnOpen.Enable()
		ui.btnHide.Enable()
		ui.btnAdd.Enable()
		ui.btnExtract.Enable()
		ui.btnDelete.Enable()
		ui.btnPassword.Enable()
		ui.btnLock.Enable()
	}
	ui.refreshList()
}

func (ui *App) refreshList() {
	if ui.v == nil || !ui.v.IsUnlocked() {
		ui.files = nil
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
	ui.fileList.Refresh()
	ui.syncStage()
	n := len(names)
	if n == 1 {
		ui.countLabel.SetText("1 item")
	} else {
		ui.countLabel.SetText(fmt.Sprintf("%d items", n))
	}
	if ui.jobs.Busy() {
		return
	}
	if n == 0 {
		ui.setStatus("Vault unlocked — empty. Add a file to get started.")
	} else {
		ui.setStatus(fmt.Sprintf("Unlocked — %d encrypted file(s).", n))
	}
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
		base := filepath.Base(path)
		ui.runAsync("Add file", "Encrypting "+base+"…", func() error {
			return ui.v.AddFile(path, false)
		}, func() {
			ui.refreshList()
			ui.jobs.Finish("Added " + base)
			ui.setStatus("Added " + base)
		})
	}, ui.win)
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
		name = ui.files[0]
	}
	nameEntry := widget.NewEntry()
	nameEntry.SetText(name)
	nameEntry.SetPlaceHolder("Filename in vault")
	form := dialog.NewForm(
		"Extract file",
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

func (ui *App) startExtract(n, outDir string, overwrite bool) {
	var outPath string
	existsConflict := false
	ui.runAsync("Extract", "Decrypting "+n+"…", func() error {
		p, err := ui.v.ExtractFile(n, outDir, overwrite)
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
						ui.setStatus("Extract cancelled — file already exists.")
						return
					}
					ui.startExtract(n, outDir, true)
				},
				ui.win,
			)
			return
		}
		ui.jobs.Finish("Extracted to " + outPath)
		ui.setStatus("Extracted to " + outPath)
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
		name = ui.files[0]
	}
	dialog.ShowConfirm(
		"Delete file",
		fmt.Sprintf("Permanently delete %q from the vault? This cannot be undone.", name),
		func(ok bool) {
			if !ok {
				return
			}
			ui.runAsync("Delete", "Removing "+name+"…", func() error {
				return ui.v.DeleteFile(name)
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
		ui.runAsync("Change password", "Re-encrypting vault under the new password…", func() error {
			return ui.v.ChangePassword(old.Text, neu.Text)
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
	items := []*widget.FormItem{widget.NewFormItem("Password", pw1)}
	var pw2 *widget.Entry
	if confirm {
		pw2 = widget.NewPasswordEntry()
		pw2.SetPlaceHolder("Confirm")
		items = append(items, widget.NewFormItem("Confirm", pw2))
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
