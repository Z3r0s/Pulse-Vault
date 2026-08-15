package ui

import "fyne.io/fyne/v2"

const (
	headerStackGap float32 = 8
	headerSideGap  float32 = 16
	navTargetWidth float32 = 228
	shellMinWidth  float32 = 560
	shellMinHeight float32 = 360
)

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

// splitFitLayout fills the body and enforces a usable min size so a
// small window scrolls instead of clipping the split.
type splitFitLayout struct{}

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

func (splitFitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	objects[0].Resize(size)
	objects[0].Move(fyne.NewPos(0, 0))
}
