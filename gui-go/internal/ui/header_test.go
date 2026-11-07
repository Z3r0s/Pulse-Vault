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

func TestSplitFitMinSize(t *testing.T) {
	inner := canvas.NewRectangle(color.White)
	inner.SetMinSize(fyne.NewSize(100, 100))
	ms := splitFitLayout{}.MinSize([]fyne.CanvasObject{inner})
	if ms.Width < shellMinWidth || ms.Height < shellMinHeight {
		t.Fatalf("min size too small: %v", ms)
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
