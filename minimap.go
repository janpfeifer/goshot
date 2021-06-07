package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"image"
	"image/color"
	"math"
)

type MiniMap struct {
	widget.BaseWidget

	gs      *GoShot
	minSize fyne.Size

	raster *canvas.Raster

	// Cache image for current dimensions/zoom/translation.
	cache                   *image.RGBA
	cacheWidth, cacheHeight int
}

func NewMiniMap(gs *GoShot) (mm *MiniMap) {
	mm = &MiniMap{
		gs: gs,
	}
	mm.raster = canvas.NewRaster(mm.draw)
	mm.SetMinSize(fyne.NewSize(200, 200))
	return
}

// Draw implements canvas.Raster Generator: it generates the image that will be drawn.
// The image should already be rendered in mm.cache, but this handles exception cases.
func (mm *MiniMap) draw(w, h int) image.Image {
	glog.V(2).Infof("draw(w=%d, h=%d)", w, h)
	if mm.cache != nil {
		if cacheW, cacheH := wh(mm.cache); cacheW == w && cacheH == h {
			// Cache is good, reuse it.
			return mm.cache
		}
	}

	// Regenerate cache.
	mm.cache = image.NewRGBA(image.Rect(0, 0, w, h))
	mm.renderCache()
	return mm.cache
}

func (mm *MiniMap) Resize(size fyne.Size) {
	glog.V(2).Infof("Resize(size={w=%g, h=%g})", size.Width, size.Height)
	mm.BaseWidget.Resize(size)
	mm.raster.Resize(size)
}

func (mm *MiniMap) SetMinSize(size fyne.Size) {
	mm.minSize = size
}

func (mm *MiniMap) MinSize() fyne.Size {
	return mm.minSize
}

func (mm *MiniMap) CreateRenderer() fyne.WidgetRenderer {
	glog.V(2).Info("CreateRenderer()")
	return mm
}

func (mm *MiniMap) Destroy() {}

func (mm *MiniMap) Layout(size fyne.Size) {
	glog.V(2).Infof("Layout: size=(w=%g, h=%g)", size.Width, size.Height)
	// Resize to given size
	mm.raster.Resize(size)
}

func (mm *MiniMap) Refresh() {
	glog.V(2).Info("Refresh()")
	mm.renderCache()
	canvas.Refresh(mm)
}

func (mm *MiniMap) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{mm.raster}
}

func (mm *MiniMap) BackgroundColor() color.Color {
	return theme.BackgroundColor()
}

func (mm *MiniMap) renderCache() {
	const bytesPerPixel = 4 // RGBA.
	w, h := wh(mm.cache)
	img := mm.gs.Screenshot
	imgW, imgH := wh(img)
	zoom := float64(imgW) / float64(w)
	zoomY := float64(imgH) / float64(h)
	if zoomY > zoom {
		zoom = zoomY
	}

	var c color.RGBA
	glog.V(2).Infof("renderCache(): cache=(w=%d, h=%d, bytes=%d), zoomFactor=%g",
		w, h, len(mm.cache.Pix), zoom)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pos := (y*w + x) * bytesPerPixel
			imgX := int(math.Round(float64(x)*zoom + 0.5))
			imgY := int(math.Round(float64(y)*zoom + 0.5))
			if imgX >= imgW || imgY >= imgH {
				// Background image.
				c = bgPattern(x, y)
			} else {
				c = img.RGBAAt(imgX, imgY)
			}
			mm.cache.Pix[pos] = c.R
			mm.cache.Pix[pos+1] = c.G
			mm.cache.Pix[pos+2] = c.B
			mm.cache.Pix[pos+3] = c.A
		}
	}
}
