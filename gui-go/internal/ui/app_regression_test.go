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
	a.Settings().SetTheme(&PulseTheme{})
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
		fyne.NewSize(640, 360),
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
		if !ui.btnCheckUpdates.Visible() || ui.btnCheckUpdates.Size().Width <= 0 {
			t.Errorf("at %v, the update action should remain available in the header", size)
		}
		if size.Height <= 360 && ui.topBar.Size().Height >= size.Height/2 {
			t.Errorf("at %v, the brand header consumes half the window: %v", size, ui.topBar.Size())
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
	for _, size := range []fyne.Size{fyne.NewSize(2560, 1440), fyne.NewSize(800, 600), fyne.NewSize(640, 360)} {
		ui.win.Resize(size)
		want := float64(sidebarWidth(size.Width) / size.Width)
		if math.Abs(ui.split.Offset-want) > 0.001 {
			t.Errorf("after resizing back to %v, split offset = %.3f, want %.3f", size, ui.split.Offset, want)
		}
	}
}

func TestCompactVaultActionsStayVisibleWithoutCoveringFileControls(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(640, 480))
	tmp := t.TempDir()
	v := vault.New(filepath.Join(tmp, "compact.pulsevault"))
	if err := v.Create("test-only-passphrase", "standard"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Lock)
	source := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(source, []byte("sample"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	ui.v = v
	ui.setUnlockedUI("compact.pulsevault")
	ui.fileList.Select(0)

	if ui.compactVaultActions.Layout == nil || ui.btnCreate.Text != "New" || ui.btnOpen.Text != "Open" || ui.btnHide.Text != "Hide" {
		t.Fatal("short windows should compact vault actions into the labeled one-row layout")
	}
	if !ui.compactVaultActions.Visible() || ui.compactVaultActions.Size().Width < ui.compactVaultActions.MinSize().Width {
		t.Fatalf("compact vault actions should fit their sidebar row: size=%v min=%v", ui.compactVaultActions.Size(), ui.compactVaultActions.MinSize())
	}
	if !ui.btnAdd.Visible() || !ui.btnAddFolder.Visible() || !ui.btnExtract.Visible() || !ui.btnExtractAll.Visible() || !ui.btnDelete.Visible() || !ui.btnPassword.Visible() || !ui.btnLock.Visible() {
		t.Fatal("all file and vault controls should remain present in the compact sidebar")
	}
	if content := ui.navScroll.Content.Size().Height; content > ui.navScroll.Size().Height+1 {
		t.Fatalf("the compact sidebar should fit its action rows without clipping: content=%v viewport=%v", content, ui.navScroll.Size().Height)
	}
}

func TestWindowMinimumSizeKeepsWorkspaceUsable(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(1120, 700))
	tmp := t.TempDir()
	v := vault.New(filepath.Join(tmp, "minimum.pulsevault"))
	if err := v.Create("test-only-passphrase", "standard"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Lock)
	source := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(source, []byte("sample"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	ui.v = v
	ui.setUnlockedUI("example.pulsevault")
	minimum := ui.root.MinSize()
	t.Logf("minimum supported content size: %v", minimum)
	if minimum.Width < 640 || minimum.Height < 480 {
		t.Fatalf("window minimum %v is too small for both panes and persistent controls", minimum)
	}
	ui.win.Resize(minimum)
	if ui.mainStage.Size().Width < 300 || ui.mainStage.Size().Height < 160 {
		t.Fatalf("minimum supported window leaves an unusable workspace: %v", ui.mainStage.Size())
	}
	stageInner := ui.mainStage.Objects[1].(*fyne.Container)
	if !stageInner.Objects[0].Visible() || ui.search.Size().Height <= 0 {
		t.Fatalf("file heading and search must remain visible at minimum size %v: stage=%v header=%v/%v search=%v", minimum, ui.mainStage.Size(), stageInner.Objects[0].Visible(), stageInner.Objects[0].Size(), ui.search.Size())
	}
	if !ui.fileList.Visible() {
		t.Fatal("the file list should remain visible at the minimum supported window size")
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
	actionsBottom := ui.compactVaultActions.Position().Y + ui.compactVaultActions.Size().Height
	if ui.search.Size().Height <= 0 || ui.fileNav.Position().Y < actionsBottom {
		t.Fatalf("revealing file controls must relayout below vault actions and show search: search=%v fileNav=%v actionGroup=%v+%v", ui.search.Size(), ui.fileNav.Position(), ui.compactVaultActions.Position(), ui.compactVaultActions.Size())
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

func TestShortcutGuardProtectsTypingAndBusyFlows(t *testing.T) {
	ui, _ := testUI(t, fyne.NewSize(800, 600))
	ui.search.Show()
	canv := ui.win.Canvas()
	canv.Focus(ui.search)
	if ui.canUseShortcut(false) {
		t.Fatal("global file actions must not fire while a text field is focused")
	}
	if !ui.canUseShortcut(true) {
		t.Fatal("Find should remain available while the search field is focused")
	}
	canv.Unfocus()
	if err := ui.jobs.Begin("Add file", "Encrypting…"); err != nil {
		t.Fatal(err)
	}
	if ui.canUseShortcut(true) {
		t.Fatal("global shortcuts must be ignored while a vault operation is running")
	}
	ui.jobs.Finish("done")
	if !ui.canUseShortcut(false) {
		t.Fatal("global shortcuts should be available again when the workspace is idle")
	}
	overlay := canvas.NewRectangle(colBg)
	canv.Overlays().Add(overlay)
	if ui.canUseShortcut(false) {
		t.Fatal("global shortcuts must not fire under an open dialog")
	}
	canv.Overlays().Remove(overlay)
}

func TestShortWindowRendersWithProductTheme(t *testing.T) {
	a := test.NewApp()
	a.Settings().SetTheme(&PulseTheme{})
	ui := &App{fyneApp: a}
	ui.win = a.NewWindow(AppTitle)
	ui.win.SetPadded(false)
	ui.win.Resize(fyne.NewSize(640, 360))
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
	if !stage.Objects[0].Visible() {
		t.Fatal("the contextual header should remain visible when the compact layout has room")
	}
	ui.win.Resize(fyne.NewSize(640, 240))
	if stage.Objects[0].Visible() {
		t.Fatal("the contextual header should collapse when the workspace is too short")
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
	if ui.btnCreate.Importance != widget.HighImportance || ui.btnLock.Importance == widget.DangerImportance {
		t.Fatal("locked-state emphasis should favor creating or opening a vault and keep Lock out of the destructive style")
	}
	if !ui.btnAdd.Disabled() || !ui.btnExtract.Disabled() || !ui.btnLock.Disabled() {
		t.Fatal("file operations and lock should be disabled while locked")
	}
	if ui.btnCancel.Visible() {
		t.Fatal("cancel should be hidden while idle")
	}
	if ui.btnCheckUpdates.Disabled() {
		t.Fatal("the optional update check should be available while the vault is locked")
	}

	ui.setBusyChromeForOperation(true, false)
	if ui.btnCancel.Visible() {
		t.Fatal("cancel should be hidden for non-cancellable work")
	}
	if !ui.btnCreate.Disabled() || !ui.btnOpen.Disabled() {
		t.Fatal("vault switching should be disabled during work")
	}
	if !ui.btnCheckUpdates.Disabled() {
		t.Fatal("the update request should not start while a vault operation is running")
	}
	ui.setBusyChromeForOperation(false, false)
	if ui.btnCheckUpdates.Disabled() {
		t.Fatal("the update action should return after the vault operation finishes")
	}

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
	if ui.pathLabel.Text != filepath.Base(vaultPath) {
		t.Fatalf("unlocked path label = %q; want only vault filename %q", ui.pathLabel.Text, filepath.Base(vaultPath))
	}
	if !ui.heroCard.Visible() || ui.fileList.Visible() || ui.btnAdd.Disabled() {
		t.Fatal("an empty unlocked vault should show its empty state and allow adding files")
	}
	if !ui.heroEmptyActions.Visible() || ui.heroLockedActions.Visible() || ui.heroAddFile.OnTapped == nil || ui.heroAddFolder.OnTapped == nil {
		t.Fatal("the empty vault should offer direct, working add-file and add-folder actions")
	}
	if ui.btnCreate.Importance == widget.HighImportance || ui.btnAdd.Importance == widget.HighImportance {
		t.Fatal("an empty unlocked vault should emphasize the direct workspace Add action instead of vault navigation")
	}
	if !ui.fileNav.Visible() || !ui.btnAdd.Visible() || !ui.btnAddFolder.Visible() {
		t.Fatal("add actions should remain available in the unlocked navigation")
	}
	ui.win.Resize(fyne.NewSize(640, 360))
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
	if ui.btnAdd.Importance != widget.HighImportance || ui.btnCreate.Importance == widget.HighImportance {
		t.Fatal("a populated vault should emphasize Add file and demote vault navigation")
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
	a := test.NewApp()
	defer a.Quit()
	rowObject, ok := newFileListRow().(*fileListRow)
	if !ok {
		t.Fatal("file row should be a Fyne widget")
	}
	if renderer := test.WidgetRenderer(rowObject); renderer == nil {
		t.Fatal("file row must provide a renderer so its contents are painted")
	} else {
		renderer.Layout(fyne.NewSize(300, 64))
		renderer.Refresh()
	}
	row := rowObject.content
	var name *widget.Label
	var namePosition fyne.Position
	var nameSize fyne.Size
	var details *canvas.Text
	var detailsPosition fyne.Position
	var detailsSize fyne.Size
	var visit func(fyne.CanvasObject, fyne.Position)
	visit = func(object fyne.CanvasObject, parent fyne.Position) {
		position := parent.Add(object.Position())
		switch child := object.(type) {
		case *widget.Label:
			name = child
			namePosition = position
			nameSize = child.Size()
		case *canvas.Text:
			details = child
			detailsPosition = position
			detailsSize = child.Size()
		case *fyne.Container:
			for _, nested := range child.Objects {
				visit(nested, position)
			}
		}
	}
	visit(row, fyne.NewPos(0, 0))
	if name == nil || details == nil {
		t.Fatal("file row should contain a filename and metadata")
	}
	longName := strings.Repeat("very-long-project-name-", 20) + ".csv"
	updateFileListRow(rowObject, longName, "2.4 MB · Added Nov 7")
	rowObject.Resize(fyne.NewSize(300, 64))
	rowObject.Refresh()
	namePosition = fyne.NewPos(0, 0)
	nameSize = fyne.NewSize(0, 0)
	detailsPosition = fyne.NewPos(0, 0)
	detailsSize = fyne.NewSize(0, 0)
	visit(row, fyne.NewPos(0, 0))
	if name.Truncation != fyne.TextTruncateEllipsis || name.Wrapping != fyne.TextWrapOff {
		t.Fatal("long file names should truncate on one line")
	}
	if details.Text != "2.4 MB · Added Nov 7" {
		t.Fatalf("row details = %q", details.Text)
	}
	if nameSize.Width < 50 || namePosition.X+nameSize.Width > detailsPosition.X+1 {
		t.Fatalf("filename has no usable space or overlaps metadata: name=%v+%v details=%v+%v", namePosition, nameSize, detailsPosition, detailsSize)
	}
	if detailsPosition.X+detailsSize.Width > row.Size().Width-12 || detailsPosition.Y+detailsSize.Height > row.Size().Height {
		t.Fatalf("metadata needs a visible right gutter and must fit vertically: details=%v+%v row=%v", detailsPosition, detailsSize, row.Size())
	}
}
