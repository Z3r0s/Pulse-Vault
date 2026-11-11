package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestStageContentLayoutCollapsesVaultPathInShortWindows(t *testing.T) {
	path := widget.NewLabel("Example.pulsevault")
	header := container.NewVBox(widget.NewLabel("Secure files"), path)
	content := container.NewVBox(widget.NewLabel("File list"))
	layout := &stageContentLayout{pathLabel: path, compactHeight: compactPathCollapseHeight}
	objects := []fyne.CanvasObject{header, content}

	layout.Layout(objects, fyne.NewSize(640, 350))
	if path.Visible() {
		t.Fatal("vault path should collapse in a short window")
	}

	layout.Layout(objects, fyne.NewSize(1120, 600))
	if !path.Visible() {
		t.Fatal("vault path should return when the stage has room")
	}
}

func TestGUISourceHasPrimaryAffordances(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(thisFile)
	appSrc, err := os.ReadFile(filepath.Join(dir, "app.go"))
	if err != nil {
		t.Fatal(err)
	}
	code := string(appSrc)
	needles := []string{
		"Create vault",
		"Open vault",
		"Hide in picture",
		"Add file",
		"Add folder",
		"Extract",
		"Save a copy",
		"Save all files",
		"Change password",
		"Lock vault",
		"Cancel",
		"Delete",
		"Lock",
		"fileList",
		"heroCard",
		"status",
		"navRail",
		"topBar",
		"Pulse-Vault",
		"PULSE-VAULT",
		"onCreate",
		"onOpen",
		"onHide",
		"CreateWithCarrier",
		"onAdd",
		"onExtract",
		"onDelete",
		"ChangePassword",
		"onLock",
		"brandMark",
		"lockBadge",
		"mainStage",
		"navRail",
		"topBar",
		"NewHSplit",
		"newBrandImage",
		"NewVScroll",
		"headerWrapLayout",
		"bindShortcuts",
		"SetOnDropped",
		"syncStage",
		"runAsync",
		"onMain",
		"fyne.Do",
		"progress",
		"setBusyChrome",
		"setBusyChromeForOperation",
		"chooseOutputPath",
		"validateOutputName",
		"ensureDestinationAbsent",
		"ShowConfirm",
		"version.Version",
		"dnspulse.org",
	}
	for _, n := range needles {
		if !strings.Contains(code, n) {
			t.Errorf("GUI source missing marker %q", n)
		}
	}
	if strings.Contains(code, "startUIPump") {
		t.Error("do not use animation pump for main-thread UI; use fyne.Do")
	}
	if strings.Contains(code, "ShowFileSave") {
		t.Error("vault destinations must be chosen without opening a truncating file writer")
	}
	if !strings.Contains(code, `runAsync("Create vault"`) {
		t.Error("create must use runAsync")
	}
	if !strings.Contains(code, `runAsync("Unlock"`) {
		t.Error("unlock must use runAsync")
	}
	if !strings.Contains(code, `runAsyncWithOptions("Add files"`) {
		t.Error("add must use cancellable progress-aware runAsyncWithOptions")
	}
	if !strings.Contains(code, `runAsyncWithOptions("Save a copy"`) {
		t.Error("saving a copy must use cancellable progress-aware runAsyncWithOptions")
	}
	if !strings.Contains(code, `runAsyncWithOptions("Save all files"`) {
		t.Error("saving all files must use cancellable progress-aware runAsyncWithOptions")
	}
	if !strings.Contains(code, "runAsyncWithOptionsAndCancel(op, pendingMsg, work, true, onOK)") {
		t.Error("progress-aware stream operations must expose the real cancellation control")
	}
	if !strings.Contains(code, "}, false, onOK)") || !strings.Contains(code, "setBusyChromeForOperation(true, cancellable)") {
		t.Error("non-cancellable operations must not show the Cancel button")
	}
	if !strings.Contains(code, `runAsync("Hide in picture"`) {
		t.Error("hide-in-picture must use runAsync")
	}
	if !strings.Contains(code, `runAsyncWithOptions("Delete"`) {
		t.Error("delete must use cancellable progress-aware runAsyncWithOptions")
	}
}

func TestThemeIsDarkFirstCustom(t *testing.T) {
	th := &PulseTheme{}
	bg := th.Color(theme.ColorNameBackground, theme.VariantLight)
	r, g, b, _ := bg.RGBA()
	if r > 0x3000 || g > 0x3000 || b > 0x3000 {
		t.Fatalf("expected near-black product chrome, got R=%d G=%d B=%d", r, g, b)
	}
	// Pulse cyan primary (not old blue-slate)
	primary := th.Color(theme.ColorNamePrimary, theme.VariantDark)
	pr, pg, pb, _ := primary.RGBA()
	// cyan: green and blue high, red low
	if pr > pg || pr > pb {
		t.Fatalf("expected cyan primary (low red), got R=%d G=%d B=%d", pr, pg, pb)
	}
}

func TestJobsHelpersExportedForReactiveShell(t *testing.T) {
	var j JobState
	if err := j.Begin("Add file", "Encrypting…"); err != nil {
		t.Fatal(err)
	}
	line := StatusLine(true, "Add file", "Encrypting…")
	if !strings.Contains(line, "Add file") {
		t.Fatal(line)
	}
	j.Finish("ok")
}
