package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"image"
	"image/color"
	"math"
)

// ViewPort is our view port for the image being edited. It's a specialized widget
// that will display the image according to zoom / window select.
//
// It is both a CanvasObject and a WidgetRenderer.
//
// Based on github.com/fyne-io/pixeledit
type ViewPort struct {
	widget.BaseWidget

	gs       *GoShot
	minSize  fyne.Size
	Log2Zoom float64

	raster *canvas.Raster

	// Cache image for current dimensions/zoom/translation.
	cache                   *image.RGBA
	cacheWidth, cacheHeight int
}

func NewViewPort(gs *GoShot) (vp *ViewPort) {
	vp = &ViewPort{
		gs: gs,
	}
	vp.raster = canvas.NewRaster(vp.draw)
	vp.SetMinSize(fyne.NewSize(100, 100))
	return
}

func (vp *ViewPort) Scrolled(ev *fyne.ScrollEvent) {
	glog.V(2).Infof("Scrolled(dx=%f, dy=%f)", ev.Scrolled.DX, ev.Scrolled.DY)
	vp.Log2Zoom += float64(ev.Scrolled.DY) / 50.0
	vp.gs.zoomEntry.SetText(fmt.Sprintf("%.3g", vp.Log2Zoom))
	vp.Refresh()
}

// Draw implements canvas.Raster Generator: it generates the image that will be drawn.
// The image should already be rendered in vp.cache, but this handles exception cases.
func (vp *ViewPort) draw(w, h int) image.Image {
	glog.V(2).Infof("draw(w=%d, h=%d)", w, h)
	if vp.cache != nil {
		if cacheW, cacheH := wh(vp.cache); cacheW == w && cacheH == h {
			// Cache is good, reuse it.
			return vp.cache
		}
	}

	// Regenerate cache.
	vp.cache = image.NewRGBA(image.Rect(0, 0, w, h))
	vp.renderCache()
	return vp.cache
}

func (vp *ViewPort) Resize(size fyne.Size) {
	glog.V(2).Infof("Resize(size={w=%g, h=%g})", size.Width, size.Height)
	vp.BaseWidget.Resize(size)
	vp.raster.Resize(size)
}

func (vp *ViewPort) SetMinSize(size fyne.Size) {
	vp.minSize = size
}

func (vp *ViewPort) MinSize() fyne.Size {
	return vp.minSize
}

func (vp *ViewPort) CreateRenderer() fyne.WidgetRenderer {
	glog.V(2).Info("CreateRenderer()")
	return vp
}

func (vp *ViewPort) Destroy() {}

func (vp *ViewPort) Layout(size fyne.Size) {
	glog.V(2).Infof("Layout: size=(w=%g, h=%g)", size.Width, size.Height)
	// Resize to given size
	vp.raster.Resize(size)
}

func (vp *ViewPort) Refresh() {
	glog.V(2).Info("Refresh()")
	vp.renderCache()
	canvas.Refresh(vp)
}

func (vp *ViewPort) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{vp.raster}
}

func (vp *ViewPort) BackgroundColor() color.Color {
	return theme.BackgroundColor()
}

// wh extracts the width and height of an image.
func wh(img image.Image) (int, int) {
	rect := img.Bounds()
	return rect.Dx(), rect.Dy()
}

func (vp *ViewPort) zoomFactor() float64 {
	return math.Exp2(-vp.Log2Zoom)
}

func (vp *ViewPort) renderCache() {
	const bytesPerPixel = 4 // RGBA.
	w, h := wh(vp.cache)
	img := vp.gs.Screenshot
	imgW, imgH := wh(img)
	zoom := vp.zoomFactor()

	var c color.RGBA
	glog.V(2).Infof("renderCache(): cache=(w=%d, h=%d, bytes=%d), zoomFactor=%g",
		w, h, len(vp.cache.Pix), zoom)
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
			vp.cache.Pix[pos] = c.R
			vp.cache.Pix[pos+1] = c.G
			vp.cache.Pix[pos+2] = c.B
			vp.cache.Pix[pos+3] = c.A
		}
	}
}

var (
	bgDark, bgLight = color.RGBA{58, 58, 58, 0xFF}, color.RGBA{84, 84, 84, 0xFF}
)

func bgPattern(x, y int) color.RGBA {
	const boxSize = 25
	if (x/boxSize)%2 == (y/boxSize)%2 {
		return bgDark
	}
	return bgLight
}
