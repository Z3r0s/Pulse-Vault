package ui

import "testing"

func TestBrandAssetEmbedded(t *testing.T) {
	if len(brandJPG) < 1000 {
		t.Fatalf("brand.jpg embed missing or tiny (%d bytes)", len(brandJPG))
	}
	img := newBrandImage(42)
	if img == nil {
		t.Fatal("newBrandImage returned nil")
	}
	if img.MinSize().Width < 40 {
		t.Fatalf("brand min size = %v", img.MinSize())
	}
}
