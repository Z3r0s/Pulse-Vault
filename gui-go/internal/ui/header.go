package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

const (
	headerStackGap            float32 = 8
	headerSideGap             float32 = 16
	navTargetWidth            float32 = 228
	shellMinWidth             float32 = 640
	shellMinHeight            float32 = 480
	stageBodyReserve          float32 = 72
	compactPathCollapseHeight float32 = 420
	compactNavHeight          float32 = 520
	resizeHysteresis          float32 = 14
)

// stageContentLayout gives the workspace priority in short windows. The
// contextual header returns as soon as there is room for it and the actions.
type stageContentLayout struct {
	pathLabel     fyne.CanvasObject
	compactHeight float32
	compact       bool
}

func (stageContentLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 80)
}

func (l *stageContentLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	header, content := objects[0], objects[1]
	if l.pathLabel != nil && l.compactHeight > 0 {
		compact := compactAtHeight(l.compact, size.Height, l.compactHeight)
		if compact != l.compact {
			l.compact = compact
			if compact {
				l.pathLabel.Hide()
			} else {
				l.pathLabel.Show()
			}
			header.Refresh()
		}
	}
	headerHeight := header.MinSize().Height
	if size.Height < headerHeight+stageBodyReserve {
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
type headerWrapLayout struct {
	stacked bool
}

func (headerWrapLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	if len(objects) == 1 {
		return objects[0].MinSize()
	}
	left := objects[0].MinSize()
	right := objects[1].MinSize()
	// The minimum size should reflect the compact one-row layout. The window
	// already has a minimum width large enough for the brand and status groups;
	// reserving their stacked height here wastes most of the workspace on short
	// windows even when Layout can place them side by side.
	return fyne.NewSize(
		left.Width+headerSideGap+right.Width,
		fyne.Max(left.Height, right.Height),
	)
}

func (l *headerWrapLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	if len(objects) == 1 {
		objects[0].Resize(size)
		objects[0].Move(fyne.NewPos(0, 0))
		return
	}
	left, right := objects[0], objects[1]
	leftSize, rightSize := left.MinSize(), right.MinSize()
	need := leftSize.Width + rightSize.Width + headerSideGap
	if l.stacked {
		if size.Width >= need+resizeHysteresis {
			l.stacked = false
		}
	} else if size.Width < need {
		l.stacked = true
	}
	if l.stacked {
		left.Resize(fyne.NewSize(fyne.Min(size.Width, leftSize.Width), leftSize.Height))
		left.Move(fyne.NewPos(0, 0))
		right.Resize(fyne.NewSize(fyne.Min(size.Width, rightSize.Width), rightSize.Height))
		right.Move(fyne.NewPos(0, leftSize.Height+headerStackGap))
		return
	}
	yL := (size.Height - leftSize.Height) / 2
	if yL < 0 {
		yL = 0
	}
	yR := (size.Height - rightSize.Height) / 2
	if yR < 0 {
		yR = 0
	}
	left.Resize(leftSize)
	left.Move(fyne.NewPos(0, yL))
	right.Resize(rightSize)
	right.Move(fyne.NewPos(size.Width-rightSize.Width, yR))
}

// splitFitLayout fills the body and keeps the split wide enough for both
// panes; each pane handles its own scrolling when the window is short.
type splitFitLayout struct {
	lastWidth     float32
	compactHeight float32
	compact       bool
	compactHide   []fyne.CanvasObject
	compactChange func(bool)
	refresh       func()
}

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
	if l.compactHeight > 0 {
		compact := compactAtHeight(l.compact, size.Height, l.compactHeight)
		if compact != l.compact {
			l.compact = compact
			for _, obj := range l.compactHide {
				if compact {
					obj.Hide()
				} else {
					obj.Show()
				}
			}
			if l.compactChange != nil {
				l.compactChange(compact)
			}
			if l.refresh != nil {
				l.refresh()
			}
		}
	}
	if split, ok := objects[0].(*container.Split); ok && size.Width > 0 && size.Width != l.lastWidth {
		split.Offset = float64(sidebarWidth(size.Width) / size.Width)
		l.lastWidth = size.Width
	}
	objects[0].Resize(size)
	objects[0].Move(fyne.NewPos(0, 0))
}

// compactAtHeight adds a small dead band around responsive breakpoints so a
// drag near the threshold does not repeatedly hide and restore interface rows.
func compactAtHeight(current bool, height, threshold float32) bool {
	if current {
		return height < threshold+resizeHysteresis
	}
	return height < threshold-resizeHysteresis
}
