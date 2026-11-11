package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func TestHeaderWrapStacksWhenNarrow(t *testing.T) {
	left := canvas.NewRectangle(color.White)
	left.SetMinSize(fyne.NewSize(280, 48))
	right := canvas.NewRectangle(color.White)
	right.SetMinSize(fyne.NewSize(180, 36))
	lay := headerWrapLayout{}
	objs := []fyne.CanvasObject{left, right}

	lay.Layout(objs, fyne.NewSize(400, 120))
	if right.Position().Y <= 0 {
		t.Fatalf("expected stacked header, right.Y=%v", right.Position().Y)
	}

	lay.Layout(objs, fyne.NewSize(900, 80))
	if right.Position().X < 600 {
		t.Fatalf("expected right group on the trailing edge, pos=%v", right.Position())
	}
	if right.Position().Y >= left.MinSize().Height {
		t.Fatalf("expected side-by-side header, right=%v", right.Position())
	}
}

func TestHeaderWrapMinSizeUsesCompactRow(t *testing.T) {
	left := canvas.NewRectangle(color.White)
	left.SetMinSize(fyne.NewSize(280, 48))
	right := canvas.NewRectangle(color.White)
	right.SetMinSize(fyne.NewSize(180, 36))
	min := (headerWrapLayout{}).MinSize([]fyne.CanvasObject{left, right})
	if min.Height != 48 {
		t.Fatalf("header min height = %v, want one-row height 48", min.Height)
	}
	if min.Width < left.MinSize().Width+headerSideGap+right.MinSize().Width {
		t.Fatalf("header min width = %v, does not fit both groups", min.Width)
	}
}

func TestHeaderWrapBreakpointHasHysteresis(t *testing.T) {
	left := canvas.NewRectangle(color.White)
	left.SetMinSize(fyne.NewSize(280, 48))
	right := canvas.NewRectangle(color.White)
	right.SetMinSize(fyne.NewSize(180, 36))
	lay := &headerWrapLayout{}
	objects := []fyne.CanvasObject{left, right}

	lay.Layout(objects, fyne.NewSize(475, 100))
	if !lay.stacked || right.Position().Y == 0 {
		t.Fatal("header should stack below the width required by both groups")
	}
	lay.Layout(objects, fyne.NewSize(480, 100))
	if !lay.stacked || right.Position().Y == 0 {
		t.Fatal("small width changes around the breakpoint should not make the header flicker")
	}
	lay.Layout(objects, fyne.NewSize(490, 100))
	if lay.stacked || right.Position().Y >= left.MinSize().Height {
		t.Fatal("header should return to one row after clearing the resize dead band")
	}
	lay.Layout(objects, fyne.NewSize(475, 100))
	if !lay.stacked {
		t.Fatal("header should stack again when the width no longer fits")
	}
}

func TestHeightBreakpointsHaveHysteresis(t *testing.T) {
	if compactAtHeight(false, compactNavHeight-10, compactNavHeight) {
		t.Fatal("a small resize near the breakpoint should not compact the navigation")
	}
	if !compactAtHeight(false, compactNavHeight-resizeHysteresis-1, compactNavHeight) {
		t.Fatal("navigation should compact after crossing the enter threshold")
	}
	if !compactAtHeight(true, compactNavHeight+resizeHysteresis-1, compactNavHeight) {
		t.Fatal("navigation should stay compact inside the resize dead band")
	}
	if compactAtHeight(true, compactNavHeight+resizeHysteresis, compactNavHeight) {
		t.Fatal("navigation should expand after clearing the resize dead band")
	}
}

func TestSplitFitMinSize(t *testing.T) {
	inner := canvas.NewRectangle(color.White)
	inner.SetMinSize(fyne.NewSize(100, 100))
	ms := splitFitLayout{}.MinSize([]fyne.CanvasObject{inner})
	if ms.Width < shellMinWidth || ms.Height < shellMinHeight {
		t.Fatalf("min size too small: %v", ms)
	}
}

func TestSplitFitCompactsSidebarActionsWhenShort(t *testing.T) {
	inner := canvas.NewRectangle(color.White)
	shortcuts := canvas.NewRectangle(color.White)
	privacy := canvas.NewRectangle(color.White)
	vaultButtons := canvas.NewRectangle(color.White)
	compact := false
	layout := &splitFitLayout{
		compactHeight: compactNavHeight,
		compactHide:   []fyne.CanvasObject{shortcuts, privacy},
		compactChange: func(value bool) { compact = value },
	}

	layout.Layout([]fyne.CanvasObject{inner}, fyne.NewSize(900, 480))
	if shortcuts.Visible() || privacy.Visible() || !vaultButtons.Visible() || !compact {
		t.Fatal("short windows should hide secondary guidance and compact the visible vault actions")
	}

	layout.Layout([]fyne.CanvasObject{inner}, fyne.NewSize(900, 700))
	if !shortcuts.Visible() || !privacy.Visible() || !vaultButtons.Visible() || compact {
		t.Fatal("full sidebar guidance and vault actions should return when the window has room")
	}
}

func TestSidebarWidthAcrossDesktopSizes(t *testing.T) {
	for _, tc := range []struct {
		canvas float32
		want   float32
	}{
		{560, 180},
		{640, 198.4},
		{800, 240},
		{1120, 268.8},
		{1366, 280},
		{1920, 280},
		{2560, 280},
	} {
		got := sidebarWidth(tc.canvas)
		if got < tc.want-0.1 || got > tc.want+0.1 {
			t.Errorf("sidebarWidth(%v) = %v; want %v", tc.canvas, got, tc.want)
		}
	}
}
