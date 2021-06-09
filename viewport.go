package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/resources"
	"image"
	"image/color"
	"image/draw"
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
	cursor  *canvas.Image

	// Cache image for current dimensions/zoom/translation.
	cache *image.RGBA

	// Dynamic dragging
	dragEvents                     chan *fyne.DragEvent
	dragStart                      fyne.Position
	dragStartViewX, dragStartViewY int
	dragSkipTap                    bool // Set at DragEnd(), because the end of the drag also triggers a tap.

	// Operations
	currentOperation OperationType

	// Crop position
	cropRect image.Rectangle
}

type OperationType int

const (
	NoOp OperationType = iota
	CropTopLeft
	CropBottomRight
	// DrawText
	// DrawArrow
	// DrawCircle
)

// Ensure ViewPort implements the following interfaces.
var (
	vpPlaceholder = &ViewPort{}
	_             = fyne.CanvasObject(vpPlaceholder)
	_             = fyne.Draggable(vpPlaceholder)
	_             = fyne.Tappable(vpPlaceholder)
)

func NewViewPort(gs *GoShot) (vp *ViewPort) {
	vp = &ViewPort{
		gs:       gs,
		cropRect: gs.OriginalScreenshot.Rect,
		cursor:   canvas.NewImageFromResource(resources.Reset),
	}
	vp.raster = canvas.NewRaster(vp.draw)
	vp.cursor.SetMinSize(fyne.NewSize(64, 64))
	vp.cursor.Resize(fyne.NewSize(100, 100))
	return
}

func (vp *ViewPort) Resize(size fyne.Size) {
	glog.V(2).Infof("Resize(size={w=%g, h=%g})", size.Width, size.Height)
	vp.BaseWidget.Resize(size)
	vp.raster.Resize(size)
	vp.cursor.Resize(fyne.NewSize(100, 100))
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
	glog.V(3).Info("Objects()")
	if vp.cursor == nil {
		return []fyne.CanvasObject{vp.raster}
	}
	return []fyne.CanvasObject{vp.raster, vp.cursor}
}

func (vp *ViewPort) BackgroundColor() color.Color {
	return theme.BackgroundColor()
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
	if vp.gs.miniMap != nil {
		vp.gs.miniMap.updateViewPortRect()
	}
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
		glog.V(2).Infof("- reuse")
		return vp.cache
	}

	// Regenerate cache.
	vp.cache = image.NewRGBA(image.Rect(0, 0, w, h))
	vp.updateViewSize()
	if vp.gs.miniMap != nil {
		vp.gs.miniMap.updateViewPortRect()
	}
	vp.renderCache()
	return vp.cache
}

// wh extracts the width and height of an image.
func wh(img image.Image) (int, int) {
	if img == nil {
		return 0, 0
	}
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
	bgDark, bgLight = color.RGBA{R: 58, G: 58, B: 58, A: 0xFF}, color.RGBA{R: 84, G: 84, B: 84, A: 0xFF}
)

func bgPattern(x, y int) color.RGBA {
	const boxSize = 25
	if (x/boxSize)%2 == (y/boxSize)%2 {
		return bgDark
	}
	return bgLight
}

// ===============================================================
// Implementation of dragging view window on ViewPort
// ===============================================================

// Dragged implements fyne.Draggable
func (vp *ViewPort) Dragged(ev *fyne.DragEvent) {
	if vp.dragEvents == nil {
		// Create a channel to send dragEvents and start goroutine to consume them sequentially.
		vp.dragEvents = make(chan *fyne.DragEvent, dragEventsQueue)
		vp.dragStart = ev.Position
		vp.dragStartViewX = vp.viewX
		vp.dragStartViewY = vp.viewY
		go vp.consumeDragEvents()
		return // No need to process first event.
	}
	vp.dragEvents <- ev
}

func (vp *ViewPort) consumeDragEvents() {
	var prevDragPos fyne.Position
	for done := false; !done; {
		var ev *fyne.DragEvent
		// Read all events in channel, until it blocks or is closed.
		consumed := 0
	drainDragEvents:
		for {
			select {
			case newEvent := <-vp.dragEvents:
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
			if ev.Position != prevDragPos {
				prevDragPos = ev.Position
				glog.V(2).Infof("consumeDragEvents(pos=%+v, consumed=%d)", ev.Position, consumed)
				vp.dragViewDelta(ev.Position.Subtract(vp.dragStart))
			}
		}
	}
	vp.dragStart = fyne.Position{}
	glog.V(2).Info("consumeDragEvents(): done")
}

func (vp *ViewPort) dragViewDelta(delta fyne.Position) {
	size := vp.Size()

	ratioX := delta.X / size.Width
	ratioY := delta.Y / size.Height

	vp.viewX = vp.dragStartViewX - int(ratioX*float32(vp.viewW)+0.5)
	vp.viewY = vp.dragStartViewY - int(ratioY*float32(vp.viewH)+0.5)
	vp.Refresh()
	vp.gs.miniMap.updateViewPortRect()
}

// DragEnd implements fyne.Draggable
func (vp *ViewPort) DragEnd() {
	glog.V(2).Infof("DragEnd(), dragEvents=%v", vp.dragEvents != nil)
	close(vp.dragEvents)
	vp.dragEvents = nil
	vp.dragSkipTap = true
}

// ===============================================================
// Implementation of a cursor on ViewPort
// ===============================================================
//func (vp *ViewPort) Set

// ===============================================================
// Implementation of operations on ViewPort
// ===============================================================

// SetOp changes the current op on the edit window. It interrupts any dragging event going on.
func (vp *ViewPort) SetOp(op OperationType) {
	if vp.dragEvents != nil {
		vp.DragEnd()
	}
	vp.currentOperation = op
}

func (vp *ViewPort) Tapped(ev *fyne.PointEvent) {
	glog.V(2).Infof("Tapped(pos=%+v), dragSkipTag=%v", ev.Position, vp.dragSkipTap)
	if vp.dragSkipTap {
		// End of a drag, we discard this tap.
		vp.dragSkipTap = false
		return
	}
	size := vp.Size()
	ratioX := ev.Position.X / size.Width
	ratioY := ev.Position.Y / size.Height
	screenshotX := int(ratioX*float32(vp.viewW) + float32(vp.viewX) + 0.5)
	screenshotY := int(ratioY*float32(vp.viewH) + float32(vp.viewY) + 0.5)

	switch vp.currentOperation {
	case NoOp:
		// Nothing ...
	case CropTopLeft:
		vp.cropTopLeft(screenshotX, screenshotY)
	case CropBottomRight:
		vp.cropBottomRight(screenshotX, screenshotY)
	}

	// After a tap
	vp.SetOp(NoOp)
}

// cropTopLeft will crop the screenshot on this position.
func (vp *ViewPort) cropTopLeft(x, y int) {
	fromRect := vp.gs.Screenshot.Rect
	crop := image.NewRGBA(image.Rect(0, 0, fromRect.Dx()-x, fromRect.Dy()-y))
	glog.V(2).Infof("cropTopLeft: new crop has size %+v", crop.Rect)
	draw.Src.Draw(crop, crop.Rect, vp.gs.Screenshot, image.Point{X: x, Y: y})
	vp.gs.Screenshot = crop
	vp.cropRect.Min = vp.cropRect.Min.Add(image.Point{X: x, Y: y})
	vp.viewX, vp.viewY = 0, 0 // Move view to cropped corner.
	glog.V(2).Infof("cropTopLeft: new cropRect is %+v", vp.cropRect)
	vp.postCrop()
}

// cropBottomRight will crop the screenshot on this position.
func (vp *ViewPort) cropBottomRight(x, y int) {
	fromRect := vp.gs.Screenshot.Rect
	crop := image.NewRGBA(image.Rect(0, 0, x, y))
	draw.Src.Draw(crop, crop.Rect, vp.gs.Screenshot, image.Point{})
	vp.gs.Screenshot = crop
	vp.cropRect.Max = vp.cropRect.Max.Sub(image.Point{X: fromRect.Dx() - x, Y: fromRect.Dy() - y})
	vp.viewX, vp.viewY = x-vp.viewW, y-vp.viewH // Move view to cropped corner.
	vp.postCrop()
}

func (vp *ViewPort) cropReset() {
	vp.gs.Screenshot = vp.gs.OriginalScreenshot
	vp.cropRect = vp.gs.Screenshot.Rect
	vp.postCrop()
	vp.gs.status.SetText(fmt.Sprintf("Reset to original screenshot of size %d x %d pixels.",
		vp.cropRect.Dx(), vp.cropRect.Dy()))
}

// postCrop refreshes elements after a change in crop.
func (vp *ViewPort) postCrop() {
	vp.updateViewSize()
	vp.renderCache()
	vp.Refresh()
	vp.gs.miniMap.updateViewPortRect()
	vp.gs.miniMap.Refresh()
	vp.gs.status.SetText(fmt.Sprintf("New crop: {%d, %d} - {%d, %d} of original screen, %d x %d pixels.",
		vp.cropRect.Min.X, vp.cropRect.Min.Y, vp.cropRect.Max.X, vp.cropRect.Max.Y,
		vp.cropRect.Dx(), vp.cropRect.Dy()))
}
