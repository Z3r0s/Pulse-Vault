package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

//go:embed brand.jpg
var brandJPG []byte

//go:embed hero-mark.svg
var heroMarkSVG []byte

func newHeroMark() *canvas.Image {
	img := canvas.NewImageFromResource(fyne.NewStaticResource("hero-mark.svg", heroMarkSVG))
	img.FillMode = canvas.ImageFillContain
	return img
}

func brandResource() fyne.Resource {
	return fyne.NewStaticResource("brand.jpg", brandJPG)
}

func newBrandImage(size float32) *canvas.Image {
	img := canvas.NewImageFromResource(brandResource())
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return img
}
