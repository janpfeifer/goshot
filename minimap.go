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

	// Application references.
	gs *GoShot
	vp *ViewPort

	// Fyne/UI related objects.
	minSize      fyne.Size
	thumbRaster  *canvas.Raster
	viewPortRect *canvas.Rectangle
	// Cache image for current dimensions/zoom/translation.
	cache *image.RGBA

	// Geometry: changed whenever window changes sizes.
	zoom           float64 // Zoom multiplier.
	thumbX, thumbY int     // Start position of thumbnail.
	thumbW, thumbH int     // Width and height of thumbnail.
}

var (
	Yellow      = color.RGBA{255, 255, 0, 255}
	Transparent = color.RGBA{0, 0, 0, 0}
)

func NewMiniMap(gs *GoShot, vp *ViewPort) (mm *MiniMap) {
	mm = &MiniMap{
		gs: gs,
		vp: vp,
	}
	mm.thumbRaster = canvas.NewRaster(mm.draw)
	mm.viewPortRect = canvas.NewRectangle(Yellow)
	mm.viewPortRect.FillColor = Transparent
	mm.viewPortRect.StrokeColor = Yellow
	mm.viewPortRect.StrokeWidth = 1.5
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
	mm.updateViewPortRect()
	mm.renderCache()
	return mm.cache
}

// drawViewPortRectangle draws a rectangle around the area that is being displayed in the
// ViewPort.
func (mm *MiniMap) drawViewPortRectangle(x, y, _, _ int) color.Color {
	if x == y {
		glog.Infof("viewPortRectangle(x=%d, y=%d)", x, y)
		return color.White
	}
	return color.RGBA{200, 200, 200, 0}
}

func (mm *MiniMap) Resize(size fyne.Size) {
	glog.V(2).Infof("Resize(size={w=%g, h=%g})", size.Width, size.Height)
	mm.BaseWidget.Resize(size)
	mm.thumbRaster.Resize(size)
	mm.updateViewPortRect()
}

func (mm *MiniMap) updateViewPortRect() {
	if mm.cache == nil {
		return
	}

	size := mm.Size()
	screenshotW, screenshotH := wh(mm.gs.Screenshot)
	ratioX := float64(mm.vp.viewX) / float64(screenshotW)
	ratioY := float64(mm.vp.viewY) / float64(screenshotH)
	ratioW := float64(mm.vp.viewW) / float64(screenshotW)
	ratioH := float64(mm.vp.viewH) / float64(screenshotH)

	pixelX := mm.thumbX + int(math.Round(ratioX*float64(mm.thumbW)))
	pixelW := int(math.Round(ratioW * float64(mm.thumbW)))
	pixelY := mm.thumbY + int(math.Round(ratioY*float64(mm.thumbH)))
	pixelH := int(math.Round(ratioH * float64(mm.thumbH)))

	w, h := wh(mm.cache)
	posX := float32(pixelX) * size.Width / float32(w)
	posY := float32(pixelY) * size.Height / float32(h)
	posW := float32(pixelW) * size.Width / float32(w)
	posH := float32(pixelH) * size.Height / float32(h)

	// Clip rectangle to minimap area.
	if posX < 0 {
		posW += posX
		posX = 0
	}
	if posY < 0 {
		posH += posY
		posY = 0
	}
	if posX+posW > size.Width {
		posW = size.Width - posX
	}
	if posY+posH > size.Height {
		posH = size.Height - posY
	}

	mm.viewPortRect.Move(fyne.NewPos(posX, posY))
	mm.viewPortRect.Resize(fyne.NewSize(posW, posH))
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
	mm.thumbRaster.Resize(size)
	mm.viewPortRect.Resize(size)
}

func (mm *MiniMap) Refresh() {
	glog.V(2).Info("Refresh()")
	mm.renderCache()
	canvas.Refresh(mm)
}

func (mm *MiniMap) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{mm.thumbRaster, mm.viewPortRect}
}

func (mm *MiniMap) BackgroundColor() color.Color {
	return theme.BackgroundColor()
}

func (mm *MiniMap) refreshGeometry() {
	w, h := wh(mm.cache)
	img := mm.gs.Screenshot
	imgW, imgH := wh(img)

	zoomX := float64(imgW) / float64(w)
	zoomY := float64(imgH) / float64(h)
	if zoomY > zoomX {
		mm.zoom = zoomY
		mm.thumbH = h
		mm.thumbY = 0
		mm.thumbW = int(math.Round(float64(imgW) / mm.zoom))
		mm.thumbX = (w - mm.thumbW) / 2
	} else {
		mm.zoom = zoomX
		mm.thumbW = w
		mm.thumbX = 0
		mm.thumbH = int(math.Round(float64(imgH) / mm.zoom))
		mm.thumbY = (h - mm.thumbH) / 2
	}
}

func (mm *MiniMap) renderCache() {
	mm.refreshGeometry()
	w, h := wh(mm.cache)
	img := mm.gs.Screenshot
	imgW, imgH := wh(img)

	const bytesPerPixel = 4 // RGBA.
	var c color.RGBA

	glog.V(2).Infof("renderCache(): cache=(w=%d, h=%d, bytes=%d), zoom=%gx",
		w, h, len(mm.cache.Pix), mm.zoom)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pos := (y*w + x) * bytesPerPixel
			imgX := int(math.Round(float64(x-mm.thumbX)*mm.zoom + 0.5))
			imgY := int(math.Round(float64(y-mm.thumbY)*mm.zoom + 0.5))
			if imgX < 0 || imgX >= imgW || imgY < 0 || imgY >= imgH {
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
