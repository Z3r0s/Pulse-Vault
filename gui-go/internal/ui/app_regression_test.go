package ui

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
)

func testUI(t *testing.T, size fyne.Size) (*App, fyne.App) {
	t.Helper()
	a := test.NewApp()
	ui := &App{fyneApp: a}
	ui.win = a.NewWindow(AppTitle)
	ui.win.SetPadded(false)
	ui.win.Resize(size)
	ui.build()
	ui.setLockedUI()
	t.Cleanup(func() {
		ui.win.Close()
		a.Quit()
	})
	return ui, a
}

func TestResponsiveWindowLayoutAcrossCommonSizes(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	sizes := []fyne.Size{
		fyne.NewSize(560, 360),
		fyne.NewSize(640, 480),
		fyne.NewSize(800, 600),
		fyne.NewSize(1024, 768),
		fyne.NewSize(1366, 768),
		fyne.NewSize(1920, 1080),
		fyne.NewSize(2560, 1440),
	}
	for _, size := range sizes {
		ui.win.Resize(size)
		wantNav := sidebarWidth(size.Width)
		gotNav := float64(ui.split.Offset) * float64(ui.split.Size().Width)
		if math.Abs(gotNav-float64(wantNav)) > 1 {
			t.Errorf("at %v, split nav width = %.1f, want %.1f", size, gotNav, wantNav)
		}
		if ui.mainStage.Size().Width < 300 {
			t.Errorf("at %v, main stage is too narrow: %v", size, ui.mainStage.Size())
		}
		if math.Abs(float64(ui.root.Size().Width-size.Width)) > 1 || math.Abs(float64(ui.root.Size().Height-size.Height)) > 1 {
			t.Errorf("at %v, root layout = %v; resize should fill the window", size, ui.root.Size())
		}
		if !ui.heroTitle.Visible() || !ui.heroActions.Visible() {
			t.Errorf("at %v, the locked-state headline and actions must remain visible", size)
		}
		if ui.heroActions.Size().Width > ui.heroCard.Size().Width {
			t.Errorf("at %v, empty-state actions exceed their card: actions=%v card=%v", size, ui.heroActions.Size(), ui.heroCard.Size())
		}
		if ui.heroActions.Position().Y+ui.heroActions.Size().Height > ui.heroCard.Size().Height {
			t.Errorf("at %v, start actions extend below the workspace", size)
		}
		if size.Width >= 800 && size.Height >= 600 && !ui.heroLogo.Visible() {
			t.Errorf("at %v, the hero artwork should return when there is room", size)
		}
	}
	ui.win.Resize(fyne.NewSize(1024, 768))
	ui.split.Offset = 0.37 // Simulate a user dragging the divider.
	ui.root.Refresh()
	if math.Abs(ui.split.Offset-0.37) > 0.001 {
		t.Fatal("a normal relayout should preserve the user's divider position")
	}
	for _, size := range []fyne.Size{fyne.NewSize(2560, 1440), fyne.NewSize(800, 600), fyne.NewSize(560, 360)} {
		ui.win.Resize(size)
		want := float64(sidebarWidth(size.Width) / size.Width)
		if math.Abs(ui.split.Offset-want) > 0.001 {
			t.Errorf("after resizing back to %v, split offset = %.3f, want %.3f", size, ui.split.Offset, want)
		}
	}
}

func TestFileSearchFiltersWithoutChangingVaultContents(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(800, 600))
	v := vault.New(filepath.Join(t.TempDir(), "search.pulsevault"))
	if err := v.Create("test-only-passphrase", "standard"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Lock)
	ui.v = v
	ui.files = []string{"Docs/Report.txt", "photos/image.png"}
	ui.filterFiles()
	if ui.search.Size().Height <= 0 || ui.fileNav.Position().Y < ui.btnHide.Position().Y+ui.btnHide.Size().Height {
		t.Fatal("revealing file controls must relayout the navigation and search field")
	}
	ui.selected = "Docs/Report.txt"
	ui.search.SetText("PHOTO")
	if len(ui.visibleFiles) != 1 || ui.visibleFiles[0] != "photos/image.png" || ui.countLabel.Text != "1 of 2 items" {
		t.Fatalf("unexpected filtered files: %v (%s)", ui.visibleFiles, ui.countLabel.Text)
	}
	if ui.selected != "" || !ui.btnExtract.Disabled() || ui.btnExtractAll.Disabled() {
		t.Fatal("filtering must clear the selection while keeping save-all available")
	}
	ui.search.SetText("missing")
	if !ui.noSearchResults.Visible() || ui.fileList.Visible() || len(ui.files) != 2 {
		t.Fatal("no matches should show a message without changing the full file list")
	}
	ui.search.SetText("")
	if len(ui.visibleFiles) != 2 || !ui.fileList.Visible() || ui.noSearchResults.Visible() {
		t.Fatal("clearing the filter should restore the complete list")
	}
}

func TestShortWindowRendersWithProductTheme(t *testing.T) {
	a := test.NewApp()
	a.Settings().SetTheme(&PulseTheme{})
	ui := &App{fyneApp: a}
	ui.win = a.NewWindow(AppTitle)
	ui.win.SetPadded(false)
	ui.win.Resize(fyne.NewSize(560, 360))
	ui.build()
	ui.setLockedUI()
	t.Cleanup(func() { ui.win.Close(); a.Quit() })
	ui.root.Refresh()
	if ui.heroCard.Size().Height <= 0 || !ui.heroTitle.Visible() || !ui.heroActions.Visible() {
		t.Fatalf("short window must retain usable start actions: hero size %v", ui.heroCard.Size())
	}
	// The software painter catches negative canvas dimensions that a layout-only
	// test can miss.
	if ui.win.Canvas().Capture() == nil {
		t.Fatal("window capture failed")
	}
	stage := ui.mainStage.Objects[1].(*fyne.Container)
	if stage.Objects[0].Visible() {
		t.Fatal("the contextual header should collapse in a short window")
	}
	ui.win.Resize(fyne.NewSize(1120, 700))
	if !stage.Objects[0].Visible() {
		t.Fatal("the contextual header should return when the window is larger")
	}
}

func TestControlStatesAcrossLockedBusyAndUnlocked(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	if !ui.btnCreate.Visible() || !ui.btnOpen.Visible() || !ui.btnHide.Visible() {
		t.Fatal("vault creation and open actions should be visible while locked")
	}
	if !ui.heroLockedActions.Visible() || ui.heroEmptyActions.Visible() || ui.fileNav.Visible() {
		t.Fatal("locked users should see only the start actions, not file-management controls")
	}
	if !ui.btnAdd.Disabled() || !ui.btnExtract.Disabled() || !ui.btnLock.Disabled() {
		t.Fatal("file operations and lock should be disabled while locked")
	}
	if ui.btnCancel.Visible() {
		t.Fatal("cancel should be hidden while idle")
	}

	ui.setBusyChromeForOperation(true, false)
	if ui.btnCancel.Visible() {
		t.Fatal("cancel should be hidden for non-cancellable work")
	}
	if !ui.btnCreate.Disabled() || !ui.btnOpen.Disabled() {
		t.Fatal("vault switching should be disabled during work")
	}
	ui.setBusyChromeForOperation(false, false)

	ui.setBusyChromeForOperation(true, true)
	if !ui.btnCancel.Visible() || ui.btnCancel.Disabled() {
		t.Fatal("cancel should be visible and enabled for cancellable work")
	}
	if ui.progress.Size().Width <= 0 || ui.progress.Size().Height <= 0 || ui.btnCancel.Size().Width <= 0 {
		t.Fatalf("busy footer controls did not receive layout space: progress=%v cancel=%v", ui.progress.Size(), ui.btnCancel.Size())
	}
	if !ui.btnAdd.Disabled() || !ui.btnExtractAll.Disabled() {
		t.Fatal("file operations should be disabled during background work")
	}
	ui.setBusyChromeForOperation(false, true)

	ctx, cancel := context.WithCancel(context.Background())
	ui.cancelJob = cancel
	ui.setBusyChromeForOperation(true, true)
	ui.cancelOperation()
	if ctx.Err() != context.Canceled || !ui.btnCancel.Disabled() || !strings.Contains(ui.status.Text, "Cancelling") {
		t.Fatal("cancelling should signal the operation, disable Cancel, and update the status")
	}
	cancel()
	ui.cancelJob = nil
	ui.setBusyChromeForOperation(false, true)

	tmp := t.TempDir()
	vaultPath := filepath.Join(tmp, "sample.pulsevault")
	source := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(source, []byte("visible test content"), 0600); err != nil {
		t.Fatal(err)
	}
	v := vault.New(vaultPath)
	if err := v.Create("test-only-passphrase", "standard"); err != nil {
		t.Fatal(err)
	}
	ui.v = v
	ui.setUnlockedUI(vaultPath)
	if !ui.heroCard.Visible() || ui.fileList.Visible() || ui.btnAdd.Disabled() {
		t.Fatal("an empty unlocked vault should show its empty state and allow adding files")
	}
	if !ui.heroEmptyActions.Visible() || ui.heroLockedActions.Visible() || ui.heroAddFile.OnTapped == nil || ui.heroAddFolder.OnTapped == nil {
		t.Fatal("the empty vault should offer direct, working add-file and add-folder actions")
	}
	if !ui.fileNav.Visible() || !ui.btnAdd.Visible() || !ui.btnAddFolder.Visible() {
		t.Fatal("add actions should remain available in the unlocked navigation")
	}
	ui.win.Resize(fyne.NewSize(560, 360))
	if ui.heroEmptyActions.Size().Width > ui.heroCard.Size().Width || !ui.heroAddFile.Visible() || !ui.heroAddFolder.Visible() {
		t.Fatalf("empty-state actions should stay visible and fit at the minimum window size: actions=%v card=%v", ui.heroEmptyActions.Size(), ui.heroCard.Size())
	}
	ui.win.Resize(fyne.NewSize(1120, 700))
	if !ui.btnExtract.Disabled() || !ui.btnExtractAll.Disabled() || !ui.btnDelete.Disabled() {
		t.Fatal("extract and delete actions should be disabled while the vault has no files")
	}
	if ui.btnExtract.Visible() || ui.btnExtractAll.Visible() || ui.btnDelete.Visible() {
		t.Fatal("empty vault navigation should hide actions that cannot do anything yet")
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	ui.setUnlockedUI(vaultPath)
	if ui.btnAdd.Disabled() || ui.btnAddFolder.Disabled() || ui.btnExtractAll.Disabled() || ui.btnLock.Disabled() {
		t.Fatal("file actions should be enabled while unlocked")
	}
	if ui.heroCard.Visible() || !ui.fileList.Visible() {
		t.Fatal("a non-empty vault should show the file list and hide the empty state")
	}
	if !ui.btnExtract.Visible() || !ui.btnExtractAll.Visible() || !ui.btnDelete.Visible() {
		t.Fatal("file-management actions should appear when the vault contains files")
	}
	if len(ui.files) != 1 || ui.files[0] != "sample.txt" {
		t.Fatalf("file list = %v", ui.files)
	}
	if !ui.btnExtract.Disabled() || !ui.btnDelete.Disabled() || ui.btnExtractAll.Disabled() {
		t.Fatal("single-file actions should wait for a file selection, while Save all remains available")
	}
	ui.fileList.Select(0)
	if ui.selected != "sample.txt" || ui.btnExtract.Disabled() || ui.btnDelete.Disabled() {
		t.Fatal("selecting a file should enable actions for that specific file")
	}
	v.Lock()
	ui.v = nil
	ui.setLockedUI()
	if !ui.heroCard.Visible() || ui.fileList.Visible() || !ui.btnAdd.Disabled() || ui.fileNav.Visible() {
		t.Fatal("locking should return to the locked empty state")
	}
}

func TestLongFileNamesStayInsideNarrowRows(t *testing.T) {
	row, ok := newFileListRow().(*fyne.Container)
	if !ok {
		t.Fatal("file row should be a layout container")
	}
	var name *widget.Label
	var encrypted *canvas.Text
	for _, child := range row.Objects {
		switch child := child.(type) {
		case *widget.Label:
			name = child
		case *canvas.Text:
			if child.Text == "ENCRYPTED" {
				encrypted = child
			}
		}
	}
	if name == nil || encrypted == nil {
		t.Fatal("file row should contain a filename and encrypted marker")
	}
	name.SetText(strings.Repeat("very-long-project-name-", 20) + ".csv")
	row.Resize(fyne.NewSize(220, 42))
	row.Refresh()
	if name.Truncation != fyne.TextTruncateEllipsis || name.Wrapping != fyne.TextWrapOff {
		t.Fatal("long file names should truncate on one line")
	}
	if name.Position().X+name.Size().Width > encrypted.Position().X+1 {
		t.Fatalf("filename overlaps the encrypted marker: name=%v+%v marker=%v+%v", name.Position(), name.Size(), encrypted.Position(), encrypted.Size())
	}
}
