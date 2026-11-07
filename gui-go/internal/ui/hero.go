package ui

import "fyne.io/fyne/v2"

// heroContentLayout keeps the locked/empty-state message usable in short
// windows by shrinking or hiding its decorative logo before content clips.
type heroContentLayout struct{}

func (heroContentLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	// A title and actions are the essential empty-state controls. Description
	// text and the logo collapse away in short windows.
	return fyne.NewSize(300, 80)
}

func (heroContentLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 4 {
		return
	}
	logo, title, body, actions := objects[0], objects[1], objects[2], objects[3]
	gap := float32(12)
	titleHeight := maxFloat32(title.MinSize().Height, 30)
	actionsHeight := maxFloat32(actions.MinSize().Height, 38)
	compact := size.Height < titleHeight+actionsHeight+3*gap+42
	showBody := !compact
	showLogo := size.Height >= titleHeight+actionsHeight+4*gap+104
	bodyHeight := float32(64)
	if size.Height < 220 {
		bodyHeight = 42
	}
	if showBody {
		body.Show()
	} else {
		body.Hide()
	}
	coreHeight := titleHeight + actionsHeight + gap
	if showBody {
		coreHeight += bodyHeight + gap
	}
	logoSize := minFloat32(108, size.Height-coreHeight-gap)
	showLogo = showLogo && logoSize >= 48
	if showLogo {
		logo.Show()
	} else {
		logo.Hide()
	}

	contentHeight := coreHeight
	if showLogo {
		contentHeight += logoSize + gap
	}
	y := (size.Height - contentHeight) / 2
	if y < 0 {
		y = 0
	}
	if showLogo {
		logo.Resize(fyne.NewSize(logoSize, logoSize))
		logo.Move(fyne.NewPos((size.Width-logoSize)/2, y))
		y += logoSize + gap
	}
	title.Resize(fyne.NewSize(size.Width, titleHeight))
	title.Move(fyne.NewPos(0, y))
	y += titleHeight + gap
	if showBody {
		bodyWidth := minFloat32(size.Width, 520)
		body.Resize(fyne.NewSize(bodyWidth, bodyHeight))
		body.Move(fyne.NewPos((size.Width-bodyWidth)/2, y))
		y += bodyHeight + gap
	}
	actionsSize := actions.MinSize()
	actions.Resize(actionsSize)
	actions.Move(fyne.NewPos((size.Width-actionsSize.Width)/2, y))
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
