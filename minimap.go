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
	thumbX, thumbY int     // Start pixel position of thumbnail.
	thumbW, thumbH int     // Pixel width and height of thumbnail.

	// Dynamic dragging
	dragEvents chan *fyne.DragEvent
}

// Ensure MiniMap implements the following interfaces.
var (
	mmPlaceholder = &MiniMap{}
	_             = fyne.CanvasObject(mmPlaceholder)
	_             = fyne.Tappable(mmPlaceholder)
	_             = fyne.Draggable(mmPlaceholder)
	_             = fyne.Tappable(mmPlaceholder)
)

var (
	Yellow      = color.RGBA{R: 255, G: 255, A: 255}
	Transparent = color.RGBA{}
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
	glog.V(2).Infof("- regenerating cache %d x %d", w, h)
	mm.cache = image.NewRGBA(image.Rect(0, 0, w, h))
	mm.updateViewPortRect()
	mm.renderCache()
	return mm.cache
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

func (mm *MiniMap) Tapped(ev *fyne.PointEvent) {
	glog.V(2).Infof("Tapped(pos=%+v)", ev.Position)
	mm.moveViewToPosition(ev.Position)
}

const dragEventsQueue = 1000 // We could make it much smaller by adding a separate goroutine, but this is simpler.

// Dragged implements fyne.Draggable
func (mm *MiniMap) Dragged(ev *fyne.DragEvent) {
	if mm.dragEvents == nil {
		// Create a channel to send dragEvents and start goroutine to consume them sequentially.
		mm.dragEvents = make(chan *fyne.DragEvent, dragEventsQueue)
		go mm.consumeDragEvents()
	}
	mm.dragEvents <- ev
}

func (mm *MiniMap) consumeDragEvents() {
	var prevPos fyne.Position
	for done := false; !done; {
		var ev *fyne.DragEvent
		// Read all events in channel, until it blocks or is closed.
		consumed := 0
	drainDragEvents:
		for {
			select {
			case newEvent := <-mm.dragEvents:
				if newEvent == nil {
					// Channel closed.
					done = true
					break drainDragEvents // Emptied the channel.
				} else {
					// New event arrived.
					consumed++
					if consumed%10 == 0 {
						glog.Info("here")
					}
					ev = newEvent
				}
			default:
				break drainDragEvents // Emptied the channel.
			}
		}
		if ev != nil {
			newPos := ev.Position
			if newPos.X != prevPos.X || newPos.Y != prevPos.Y {
				prevPos = newPos
				glog.V(2).Infof("consumeDragEvents(pos=%+v, consumed=%d)", newPos, consumed)
				mm.moveViewToPosition(newPos)
			}
		}
	}
	glog.V(2).Info("consumeDragEvents(): done")
}

// DragEnd implements fyne.Draggable
func (mm *MiniMap) DragEnd() {
	close(mm.dragEvents)
	mm.dragEvents = nil
}

func (mm *MiniMap) moveViewToPosition(pos fyne.Position) {
	size := mm.Size()
	pixW, pixH := wh(mm.cache)
	screenshotW, screenshotH := wh(mm.gs.Screenshot)

	pixX := pos.X * float32(pixW) / size.Width
	pixY := pos.Y * float32(pixH) / size.Height

	ratioX := (pixX - float32(mm.thumbX)) / float32(mm.thumbW)
	ratioY := (pixY - float32(mm.thumbY)) / float32(mm.thumbH)

	glog.V(2).Infof("- pos ratio: (%g, %g)", ratioX, ratioY)

	mm.vp.viewX = int(ratioX*float32(screenshotW) - float32(mm.vp.viewW)/2 + 0.5)
	mm.vp.viewY = int(ratioY*float32(screenshotH) - float32(mm.vp.viewH)/2 + 0.5)
	mm.vp.Refresh()
	mm.updateViewPortRect()
	// mm.viewPortRect.Refresh()
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
