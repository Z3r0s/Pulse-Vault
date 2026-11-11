package ui

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2/theme"
)

func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	linear := func(v uint32) float64 {
		x := float64(v) / 65535
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func TestPrimaryButtonTextContrast(t *testing.T) {
	productTheme := &PulseTheme{}
	foreground := relativeLuminance(productTheme.Color(theme.ColorNameForegroundOnPrimary, 0))
	background := relativeLuminance(productTheme.Color(theme.ColorNamePrimary, 0))
	if foreground > background {
		foreground, background = background, foreground
	}
	if ratio := (background + 0.05) / (foreground + 0.05); ratio < 4.5 {
		t.Fatalf("primary button text contrast = %.2f, want at least 4.5", ratio)
	}
}
