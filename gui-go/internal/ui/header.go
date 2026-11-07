package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

const (
	headerStackGap float32 = 8
	headerSideGap  float32 = 16
	navTargetWidth float32 = 228
	shellMinWidth  float32 = 560
	shellMinHeight float32 = 160
)

// stageContentLayout gives the workspace priority in short windows. The
// contextual header returns as soon as there is room for it and the actions.
type stageContentLayout struct{}

func (stageContentLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 80)
}

func (stageContentLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	header, content := objects[0], objects[1]
	headerHeight := header.MinSize().Height
	if size.Height < headerHeight+120 {
		header.Hide()
		headerHeight = 0
	} else {
		header.Show()
		header.Move(fyne.NewPos(0, 0))
		header.Resize(fyne.NewSize(size.Width, headerHeight))
	}
	remaining := fyne.Max(0, size.Height-headerHeight)
	content.Move(fyne.NewPos(0, headerHeight))
	content.Resize(fyne.NewSize(fyne.Max(0, size.Width), remaining))
}

// headerWrapLayout places left/right header groups side by side when they
// fit, and stacks them when the window is too narrow so chips are not clipped.
type headerWrapLayout struct{}

func (headerWrapLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	if len(objects) == 1 {
		return objects[0].MinSize()
	}
	left := objects[0].MinSize()
	right := objects[1].MinSize()
	// Always reserve stacked height so a shrink cannot clip the chips.
	return fyne.NewSize(
		fyne.Max(left.Width, right.Width),
		left.Height+headerStackGap+right.Height,
	)
}

func (headerWrapLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	if len(objects) == 1 {
		objects[0].Resize(size)
		objects[0].Move(fyne.NewPos(0, 0))
		return
	}
	left, right := objects[0], objects[1]
	l, r := left.MinSize(), right.MinSize()
	need := l.Width + r.Width + headerSideGap
	if size.Width < need {
		left.Resize(fyne.NewSize(fyne.Min(size.Width, l.Width), l.Height))
		left.Move(fyne.NewPos(0, 0))
		right.Resize(fyne.NewSize(fyne.Min(size.Width, r.Width), r.Height))
		right.Move(fyne.NewPos(0, l.Height+headerStackGap))
		return
	}
	yL := (size.Height - l.Height) / 2
	if yL < 0 {
		yL = 0
	}
	yR := (size.Height - r.Height) / 2
	if yR < 0 {
		yR = 0
	}
	left.Resize(l)
	left.Move(fyne.NewPos(0, yL))
	right.Resize(r)
	right.Move(fyne.NewPos(size.Width-r.Width, yR))
}

// splitFitLayout fills the body and keeps the split wide enough for both
// panes; each pane handles its own scrolling when the window is short.
type splitFitLayout struct{ lastWidth float32 }

func sidebarWidth(available float32) float32 {
	if available < 800 {
		width := available * 0.31
		if width < 180 {
			width = 180
		}
		if width > 240 {
			width = 240
		}
		if max := available * 0.45; width > max {
			width = max
		}
		return width
	}
	width := available * 0.24
	if width < 240 {
		width = 240
	}
	if width > 280 {
		width = 280
	}
	if max := available * 0.45; width > max {
		width = max
	}
	return width
}

func (splitFitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(shellMinWidth, shellMinHeight)
	}
	ms := objects[0].MinSize()
	if ms.Width < shellMinWidth {
		ms.Width = shellMinWidth
	}
	if ms.Height < shellMinHeight {
		ms.Height = shellMinHeight
	}
	return ms
}

func (l *splitFitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	if split, ok := objects[0].(*container.Split); ok && size.Width > 0 && size.Width != l.lastWidth {
		split.Offset = float64(sidebarWidth(size.Width) / size.Width)
		l.lastWidth = size.Width
	}
	objects[0].Resize(size)
	objects[0].Move(fyne.NewPos(0, 0))
}
