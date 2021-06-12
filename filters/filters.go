package filters

import (
	"image"
	"image/color"
)

type filterImage struct {
	source image.Image
	atFn   func(x, y int, under color.Color) color.Color
}

// ColorModel returns the Image's color model.
func (f *filterImage) ColorModel() color.Model { return f.source.ColorModel() }

// Bounds returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
func (f *filterImage) Bounds() image.Rectangle { return f.source.Bounds() }

// At returns the color of the pixel at (x, y).
// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
func (f *filterImage) At(x, y int) color.Color {
	return f.atFn(x, y, f.source.At(x, y))
}
