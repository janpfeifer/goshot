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

	// gs points back to application object.
	gs *GoShot

	// Geometry of what is being displayed:
	// Log2Zoom is the log2 of the zoom multiplier, it's what we show to the user.
	Log2Zoom float64
	// Are of the screenshot that is visible in the current window: these are the start (viewX, viewY)
	// and sizes in gs.screenshot pixels -- each may be zoomed in/out when displaying.
	viewX, viewY, viewW, viewH int

	// Fyne objects.
	minSize fyne.Size
	raster  *canvas.Raster
	// Cache image for current dimensions/zoom/translation.
	cache *image.RGBA
}

func NewViewPort(gs *GoShot) (vp *ViewPort) {
	vp = &ViewPort{
		gs: gs,
	}
	vp.raster = canvas.NewRaster(vp.draw)
	vp.SetMinSize(fyne.NewSize(100, 100))
	return
}

// PixelSize returns the size in pixels of the this CanvasObject, based on the last request to redraw.
func (vp *ViewPort) PixelSize() (x, y int) {
	if vp.cache == nil {
		return 0, 0
	}
	return wh(vp.cache)
}

// PosToPixel converts from the undocumented Fyne screen float dimension to actual number of pixels
// position in the image.
func (vp *ViewPort) PosToPixel(pos fyne.Position) (x, y int) {
	fyneSize := vp.Size()
	pixelW, pixelH := vp.PixelSize()
	x = int((pos.X/fyneSize.Width)*float32(pixelW) + 0.5)
	y = int((pos.Y/fyneSize.Height)*float32(pixelH) + 0.5)
	return
}

func (vp *ViewPort) Scrolled(ev *fyne.ScrollEvent) {
	glog.V(2).Infof("Scrolled(dx=%f, dy=%f, position=%+v)", ev.Scrolled.DX, ev.Scrolled.DY, ev.Position)
	size := vp.Size()
	glog.V(2).Infof("- Size=%+v", vp.Size())
	w, h := wh(vp.cache)
	glog.V(2).Infof("- PxSize=(%d, %d)", w, h)
	pixelX, pixelY := vp.PosToPixel(ev.Position)
	glog.V(2).Infof("- Pixel position: (%d, %d)", pixelX, pixelY)

	// We want to scroll, but preserve the pixel from the screenshot being viewed at the mouse position.
	ratioX := ev.Position.X / size.Width
	ratioY := ev.Position.Y / size.Height
	screenshotX := int(ratioX*float32(vp.viewW) + float32(vp.viewX) + 0.5)
	screenshotY := int(ratioY*float32(vp.viewH) + float32(vp.viewY) + 0.5)
	glog.V(2).Infof("- Screenshot position: (%d, %d)", screenshotX, screenshotY)

	// Update zoom.
	vp.Log2Zoom += float64(ev.Scrolled.DY) / 50.0
	vp.gs.zoomEntry.SetText(fmt.Sprintf("%.3g", vp.Log2Zoom))

	// Update geometry.
	vp.updateViewSize()
	vp.viewX = screenshotX - int(ratioX*float32(vp.viewW)+0.5)
	vp.viewY = screenshotY - int(ratioY*float32(vp.viewH)+0.5)
	vp.Refresh()
}

func (vp *ViewPort) updateViewSize() {
	zoom := vp.zoom()
	pixelW, pixelH := vp.PixelSize()
	vp.viewW = int(float64(pixelW)*zoom + 0.5)
	vp.viewH = int(float64(pixelH)*zoom + 0.5)
}

// Draw implements canvas.Raster Generator: it generates the image that will be drawn.
// The image should already be rendered in vp.cache, but this handles exception cases.
func (vp *ViewPort) draw(w, h int) image.Image {
	glog.V(2).Infof("draw(w=%d, h=%d)", w, h)
	currentW, currentH := vp.PixelSize()
	if currentW == w && currentH == h {
		// Cache is good, reuse it.
		return vp.cache
	}

	// Regenerate cache.
	vp.cache = image.NewRGBA(image.Rect(0, 0, w, h))
	vp.updateViewSize()
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

func (vp *ViewPort) zoom() float64 {
	return math.Exp2(-vp.Log2Zoom)
}

func (vp *ViewPort) renderCache() {
	const bytesPerPixel = 4 // RGBA.
	w, h := wh(vp.cache)
	img := vp.gs.Screenshot
	imgW, imgH := wh(img)
	zoom := vp.zoom()

	var c color.RGBA
	glog.V(2).Infof("renderCache(): cache=(w=%d, h=%d, bytes=%d), zoom=%g, viewX=%d, viewY=%d, viewW=%d, viewH=%d",
		w, h, len(vp.cache.Pix), zoom, vp.viewX, vp.viewY, vp.viewW, vp.viewH)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pos := (y*w + x) * bytesPerPixel
			imgX := int(math.Round(float64(x)*zoom)) + vp.viewX
			imgY := int(math.Round(float64(y)*zoom)) + vp.viewY
			if imgX < 0 || imgX >= imgW || imgY < 0 || imgY >= imgH {
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
