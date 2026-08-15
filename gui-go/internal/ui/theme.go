package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// PulseTheme is the product theme: obsidian + pulse-cyan.
// Distinct from the retired Python Yaru-orange and the early Go blue-slate shell.
type PulseTheme struct{}

var _ fyne.Theme = (*PulseTheme)(nil)

// Brand palette — high-contrast “security console”
var (
	colBg         = color.NRGBA{R: 0x07, G: 0x09, B: 0x0e, A: 0xff} // near-black
	colPanel      = color.NRGBA{R: 0x0f, G: 0x14, B: 0x1c, A: 0xff}
	colSidebar    = color.NRGBA{R: 0x0a, G: 0x0e, B: 0x16, A: 0xff}
	colCard       = color.NRGBA{R: 0x12, G: 0x18, B: 0x24, A: 0xff}
	colButton     = color.NRGBA{R: 0x16, G: 0x22, B: 0x32, A: 0xff}
	colHover      = color.NRGBA{R: 0x1e, G: 0x3a, B: 0x48, A: 0xff}
	colInput      = color.NRGBA{R: 0x0c, G: 0x12, B: 0x1c, A: 0xff}
	colFg         = color.NRGBA{R: 0xf0, G: 0xf6, B: 0xfa, A: 0xff}
	colMuted      = color.NRGBA{R: 0x7a, G: 0x8e, B: 0x9e, A: 0xff}
	colPrimary    = color.NRGBA{R: 0x00, G: 0xe5, B: 0xc0, A: 0xff} // pulse cyan
	colAccent     = color.NRGBA{R: 0x3d, G: 0xff, B: 0xd0, A: 0xff}
	colSep        = color.NRGBA{R: 0x1a, G: 0x2a, B: 0x38, A: 0xff}
	colSuccess    = color.NRGBA{R: 0x3d, G: 0xe0, B: 0x9a, A: 0xff}
	colError      = color.NRGBA{R: 0xff, G: 0x4d, B: 0x6d, A: 0xff}
	colWarning    = color.NRGBA{R: 0xff, G: 0xc4, B: 0x57, A: 0xff}
	colDisabled   = color.NRGBA{R: 0x1a, G: 0x22, B: 0x2c, A: 0xff}
	colLockRed    = color.NRGBA{R: 0xff, G: 0x6b, B: 0x7a, A: 0xff}
	colUnlockTeal = color.NRGBA{R: 0x00, G: 0xe5, B: 0xc0, A: 0xff}
	colTopBar     = color.NRGBA{R: 0x08, G: 0x0c, B: 0x14, A: 0xff}
)

func (t *PulseTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	_ = variant // always dark product chrome
	switch name {
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameButton:
		return colButton
	case theme.ColorNameDisabledButton:
		return colDisabled
	case theme.ColorNameDisabled:
		return colMuted
	case theme.ColorNameForeground:
		return colFg
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNamePrimary:
		return colPrimary
	case theme.ColorNameHover:
		return colHover
	case theme.ColorNameInputBackground:
		return colInput
	case theme.ColorNameInputBorder:
		return colSep
	case theme.ColorNameMenuBackground:
		return colPanel
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x04, G: 0x06, B: 0x0a, A: 0xf5}
	case theme.ColorNameSeparator:
		return colSep
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
	case theme.ColorNameSuccess:
		return colSuccess
	case theme.ColorNameError:
		return colError
	case theme.ColorNameWarning:
		return colWarning
	case theme.ColorNameFocus:
		return colAccent
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x00, G: 0xe5, B: 0xc0, A: 0x33}
	case theme.ColorNameHeaderBackground:
		return colTopBar
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (t *PulseTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *PulseTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *PulseTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInnerPadding:
		return 14
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 26
	case theme.SizeNameSubHeadingText:
		return 17
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInlineIcon:
		return 22
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	default:
		return theme.DefaultTheme().Size(name)
	}
}
