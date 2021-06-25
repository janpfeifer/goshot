package screenshot

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/filters"
	"github.com/janpfeifer/goshot/resources"
	"image"
	"image/color"
	"math"
	"strconv"
)

// ViewPort is our view port for the image being edited. It's a specialized widget
// that will display the image according to zoom / window select.
//
// It is both a CanvasObject and a WidgetRenderer -- the abstractions in Fyne are
// not clear, but when those were implemented it worked (mostly copy&paste code).
//
// Loosely based on github.com/fyne-io/pixeledit
type ViewPort struct {
	widget.BaseWidget

	// gs points back to application object.
	gs *GoShot

	// Geometry of what is being displayed:
	// Log2Zoom is the log2 of the zoom multiplier, it's what we show to the user. It
	// is set by the "zoomEntry" field in the UI
	Log2Zoom float64

	// Thickness of stroke drawing circles and arrows. Set by the corresponding UI element.
	Thickness float64

	// DrawingColor is used on all new drawing operation. BackgroundColor is used
	// for the background of text.
	DrawingColor, BackgroundColor color.Color

	// FontSize is the last used font size.
	FontSize float64

	// Are of the screenshot that is visible in the current window: these are the start (viewX, viewY)
	// and sizes in gs.screenshot pixels -- each may be zoomed in/out when displaying.
	viewX, viewY, viewW, viewH int

	// Fyne objects.
	minSize fyne.Size
	raster  *canvas.Raster

	cursor                                            *canvas.Image
	cursorCropTopLeft, cursorCropBottomRight          *canvas.Image
	cursorDrawCircle, cursorDrawArrow, cursorDrawText *canvas.Image

	mouseIn         bool // Whether the mouse is over ViewPort.
	mouseMoveEvents chan fyne.Position

	// Cache image for current dimensions/zoom/translation.
	cache *image.RGBA

	// Dynamic dragging
	dragEvents                     chan *fyne.DragEvent
	dragStart                      fyne.Position
	dragStartViewX, dragStartViewY int
	dragSkipTap                    bool // Set at DragEnd(), because the end of the drag also triggers a tap.

	// Operations
	currentOperation OperationType
	currentCircle    *filters.Circle // Circle being dragged, only used when currentOperation==DrawCircle.
	currentArrow     *filters.Arrow  // Circle being dragged, only used when currentOperation==DrawCircle.
	fyne.ShortcutHandler
}

type OperationType int

const (
	NoOp OperationType = iota
	CropTopLeft
	CropBottomRight
	DrawCircle
	DrawArrow
	DrawText
)

// Ensure ViewPort implements the following interfaces.
var (
	vpPlaceholder = &ViewPort{}
	_             = fyne.CanvasObject(vpPlaceholder)
	_             = fyne.Draggable(vpPlaceholder)
	_             = fyne.Tappable(vpPlaceholder)
	_             = desktop.Hoverable(vpPlaceholder)
)

func NewViewPort(gs *GoShot) (vp *ViewPort) {
	prefOrFloat := func(pref string, defaultValue float64) (value float64) {
		value = gs.App.Preferences().Float(pref)
		if value == 0 {
			value = defaultValue
		}
		return
	}

	vp = &ViewPort{
		gs:                    gs,
		cursorCropTopLeft:     canvas.NewImageFromResource(resources.CropTopLeft),
		cursorCropBottomRight: canvas.NewImageFromResource(resources.CropBottomRight),
		cursorDrawCircle:      canvas.NewImageFromResource(resources.DrawCircle),
		cursorDrawArrow:       canvas.NewImageFromResource(resources.DrawArrow),
		cursorDrawText:        canvas.NewImageFromResource(resources.DrawText),
		mouseMoveEvents:       make(chan fyne.Position, 1000),

		FontSize:  prefOrFloat(FontSizePreference, 16*float64(gs.Win.Canvas().Scale())),
		Thickness: prefOrFloat(ThicknessPreference, 3.0),

		DrawingColor:    gs.GetColorPreference(DrawingColorPreference, Red),
		BackgroundColor: gs.GetColorPreference(BackgroundColorPreference, Transparent),
	}
	go vp.consumeMouseMoveEvents()
	vp.raster = canvas.NewRaster(vp.draw)
	return
}

const (
	BackgroundColorPreference = "BackgroundColor"
	DrawingColorPreference    = "DrawingColor"
	FontSizePreference        = "FontSize"
	ThicknessPreference       = "Thickness"
)

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
	glog.V(3).Info("Objects()")
	if vp.cursor == nil || !vp.mouseIn {
		return []fyne.CanvasObject{vp.raster}
	}
	return []fyne.CanvasObject{vp.raster, vp.cursor}
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
		glog.V(2).Infof("Dragged(): start new drag for Op=%d", vp.currentOperation)
		// Create a channel to send dragEvents and start goroutine to consume them sequentially.
		vp.dragEvents = make(chan *fyne.DragEvent, dragEventsQueue)
		vp.dragStart = ev.Position
		vp.dragStartViewX = vp.viewX
		vp.dragStartViewY = vp.viewY
		go vp.consumeDragEvents()

		startX, startY := vp.screenshotPos(vp.dragStart)
		startX += vp.gs.CropRect.Min.X
		startY += vp.gs.CropRect.Min.Y

		switch vp.currentOperation {
		case NoOp, CropTopLeft, CropBottomRight, DrawText:
			// Drag the image around, nothing to do to start.
		case DrawCircle:
			glog.V(2).Infof("Tapped(): draw a circle starting at (%d, %d)", startX, startY)
			vp.currentCircle = filters.NewCircle(image.Rectangle{
				Min: image.Point{X: startX, Y: startY},
				Max: image.Point{X: startX + 5, Y: startY + 5},
			}, vp.DrawingColor, vp.Thickness)
			vp.gs.Filters = append(vp.gs.Filters, vp.currentCircle)
			vp.gs.ApplyFilters(false)
		case DrawArrow:
			glog.V(2).Infof("Tapped(): draw an arrow starting at (%d, %d)", startX, startY)
			vp.currentArrow = filters.NewArrow(
				image.Point{X: startX, Y: startY},
				image.Point{X: startX + 1, Y: startY + 1},
				vp.DrawingColor, vp.Thickness)
			vp.gs.Filters = append(vp.gs.Filters, vp.currentArrow)
			vp.gs.ApplyFilters(false)
		}

		return // No need to process first event.
	}
	vp.dragEvents <- ev
	vp.mouseMoveEvents <- ev.Position // Also emits a mouse move event.
}

func (vp *ViewPort) consumeDragEvents() {
	var prevDragPos fyne.Position
	for done := false; !done; {
		// Wait for something to happen.
		ev := <-vp.dragEvents
		if ev == nil {
			// All done.
			break
		}

		// Read all events in channel, until it blocks or is closed.
		consumed := 0
	drainDragEvents:
		for {
			select {
			case newEvent := <-vp.dragEvents:
				if newEvent == nil {
					// Channel closed, but we still need to process last event.
					done = true
					break drainDragEvents // Emptied the channel.
				} else {
					// New event arrived.
					consumed++
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
				vp.doDragThrottled(ev)
			}
		}
	}
	vp.dragStart = fyne.Position{}
	glog.V(2).Info("consumeDragEvents(): done")
}

// doDragThrottled is called sequentially, dropping drag events in between each call. So
// each time it is called with the latest DragEvent, dropping those that happened in between
// the previous call.
func (vp *ViewPort) doDragThrottled(ev *fyne.DragEvent) {
	switch vp.currentOperation {
	case NoOp, CropTopLeft, CropBottomRight, DrawText:
		// Drag the image around
		vp.dragViewDelta(ev.Position.Subtract(vp.dragStart))
	case DrawCircle:
		vp.dragCircle(ev.Position)
	case DrawArrow:
		vp.dragArrow(ev.Position)
	}
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

func (vp *ViewPort) dragCircle(toPos fyne.Position) {
	if vp.currentCircle == nil {
		glog.Errorf("dragCircle(): dragCircle event, but none has been started yet!?")
	}
	startX, startY := vp.screenshotPos(vp.dragStart)
	startX += vp.gs.CropRect.Min.X
	startY += vp.gs.CropRect.Min.Y
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.gs.CropRect.Min.X
	toY += vp.gs.CropRect.Min.Y
	vp.currentCircle.SetDim(image.Rectangle{
		Min: image.Point{X: startX, Y: startY},
		Max: image.Point{X: toX, Y: toY},
	}.Canon())
	glog.V(2).Infof("dragCircle(): draw a circle in %+v", vp.currentCircle)
	vp.gs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

func (vp *ViewPort) dragArrow(toPos fyne.Position) {
	if vp.currentArrow == nil {
		glog.Errorf("dragArrow(): dragArrow event, but none has been started yet!?")
	}
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.gs.CropRect.Min.X
	toY += vp.gs.CropRect.Min.Y
	vp.currentArrow.SetPoints(vp.currentArrow.From, image.Point{X: toX, Y: toY})
	glog.V(2).Infof("dragArrow(): draw an arrow in %+v", vp.currentArrow)
	vp.gs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

// DragEnd implements fyne.Draggable
func (vp *ViewPort) DragEnd() {
	glog.V(2).Infof("DragEnd(), dragEvents=%v", vp.dragEvents != nil)
	close(vp.dragEvents)

	switch vp.currentOperation {
	case NoOp, CropTopLeft, CropBottomRight, DrawText:
		// Drag the image around, nothing to do to start.
	case DrawCircle, DrawArrow:
		vp.gs.ApplyFilters(true)
	}
	vp.dragEvents = nil
	vp.dragSkipTap = true

	switch vp.currentOperation {
	case NoOp, CropTopLeft, CropBottomRight, DrawText:
		// Nothing to do
	case DrawCircle, DrawArrow:
		vp.currentCircle = nil
		vp.currentArrow = nil
		vp.gs.status.SetText("Drawing done, use Control+Z to undo.")
		vp.SetOp(NoOp)
	}
}

// ===============================================================
// Implementation of a cursor on ViewPort
// ===============================================================
//func (vp *ViewPort) Set

// MouseIn implements desktop.Hoverable.
func (vp *ViewPort) MouseIn(ev *desktop.MouseEvent) {
	vp.mouseIn = true
	if vp.cursor != nil {
		vp.cursor.Move(ev.Position)
	}
}

// MouseMoved implements desktop.Hoverable.
func (vp *ViewPort) MouseMoved(ev *desktop.MouseEvent) {
	if vp.cursor != nil {
		// Send event to channel, it will only be acted on in
		// vp.processMouseMoveEvent.
		vp.mouseMoveEvents <- ev.Position
	}
}

// MouseOut implements desktop.Hoverable.
func (vp *ViewPort) MouseOut() {
	vp.mouseIn = false
}

// processMouseMoveEvent is the function that actually acts on a
// mouse movement event.
func (vp *ViewPort) processMouseMoveEvent(pos fyne.Position) {
	if vp.cursor != nil {
		vp.cursor.Move(pos)
		vp.Refresh()
	}
}

// consumeMouseMoveEvents runs on a separate GoRoutine and
// drains the mouse movement events before acting on the
// last of them.
func (vp *ViewPort) consumeMouseMoveEvents() {
	// vp.mouseMoveEvents is only closed if app is exiting.
	for {
		// Wait for something to happen.
		ev, ok := <-vp.mouseMoveEvents
		if !ok {
			return
		}

		// Read all events in channel, until it blocks or is closed.
		consumed := 0
	mouseMoveEventsLoop:
		for {
			select {
			case newEvent, ok := <-vp.mouseMoveEvents:
				if !ok {
					return
				}

				// New event arrived.
				consumed++
				ev = newEvent
			default:
				break mouseMoveEventsLoop
			}
		}
		vp.processMouseMoveEvent(ev)
	}
}

// ===============================================================
// Implementation of operations on ViewPort
// ===============================================================
var cursorSize = fyne.NewSize(32, 32)

// SetOp changes the current op on the edit window. It interrupts any dragging event going on.
func (vp *ViewPort) SetOp(op OperationType) {
	if vp.dragEvents != nil {
		vp.DragEnd()
	}
	vp.currentOperation = op
	switch op {
	case NoOp:
		if vp.cursor != nil {
			vp.cursor = nil
			vp.Refresh()
		}

	case CropTopLeft:
		vp.cursor = vp.cursorCropTopLeft
		vp.cursor.Resize(cursorSize)

	case CropBottomRight:
		vp.cursor = vp.cursorCropBottomRight
		vp.cursor.Resize(cursorSize)

	case DrawCircle:
		vp.cursor = vp.cursorDrawCircle
		vp.cursor.Resize(cursorSize)
		vp.gs.status.SetText("Click and drag to draw circle!")

	case DrawArrow:
		vp.cursor = vp.cursorDrawArrow
		vp.cursor.Resize(cursorSize)
		vp.gs.status.SetText("Click and drag from start to end (point side) to draw an arrow!")

	case DrawText:
		vp.cursor = vp.cursorDrawText
		vp.cursor.Resize(cursorSize)
		vp.gs.status.SetText("Click to define center location of text.")
	}
}

// screenshotCoord returns the screenshot position for the given
// position in the canvas.
func (vp *ViewPort) screenshotPos(pos fyne.Position) (x, y int) {
	size := vp.Size()
	ratioX := pos.X / size.Width
	ratioY := pos.Y / size.Height
	x = int(ratioX*float32(vp.viewW) + float32(vp.viewX) + 0.5)
	y = int(ratioY*float32(vp.viewH) + float32(vp.viewY) + 0.5)
	return
}

func (vp *ViewPort) Tapped(ev *fyne.PointEvent) {
	glog.V(2).Infof("Tapped(pos=%+v, op=%d), dragSkipTag=%v", ev.Position, vp.currentOperation, vp.dragSkipTap)
	if vp.dragSkipTap {
		// End of a drag, we discard this tap.
		vp.dragSkipTap = false
		return
	}
	screenshotX, screenshotY := vp.screenshotPos(ev.Position)
	screenshotPoint := image.Point{X: screenshotX, Y: screenshotY}
	absolutePoint := screenshotPoint.Add(vp.gs.CropRect.Min)

	switch vp.currentOperation {
	case NoOp:
		// Nothing ...
	case CropTopLeft:
		vp.cropTopLeft(screenshotX, screenshotY)
	case CropBottomRight:
		vp.cropBottomRight(screenshotX, screenshotY)
	case DrawCircle, DrawArrow:
		vp.gs.status.SetText("You must drag to draw a arrow/circle.")
	case DrawText:
		vp.createTextFilter(absolutePoint)
	}

	// After a tap
	vp.SetOp(NoOp)
}

func (vp *ViewPort) createTextFilter(center image.Point) {
	var form dialog.Dialog
	textEntry := widget.NewMultiLineEntry()
	textEntry.Resize(fyne.NewSize(400, 80))
	fontSize := widget.NewEntry()
	fontSize.SetText(fmt.Sprintf("%g", vp.FontSize))
	fontSize.Validator = validation.NewRegexp(`\d`, "Must contain a number")
	bgColorRect := canvas.NewRectangle(vp.BackgroundColor)
	bgColorRect.SetMinSize(fyne.NewSize(200, 20))
	picker := dialog.NewColorPicker(
		"Pick a Color", "Select background color for text",
		func(c color.Color) {
			vp.BackgroundColor = c
			vp.gs.SetColorPreference(BackgroundColorPreference, c)
			bgColorRect.FillColor = vp.BackgroundColor
			bgColorRect.Refresh()
			form.Refresh()
		},
		vp.gs.Win)
	backgroundEntry := container.NewHBox(
		// Set color button.
		widget.NewButtonWithIcon("", resources.ColorWheel, func() { picker.Show() }),
		// No color button
		widget.NewButtonWithIcon("", resources.Reset, func() {
			vp.BackgroundColor = Transparent
			bgColorRect.FillColor = vp.BackgroundColor
			bgColorRect.Refresh()
		}),
		bgColorRect,
	)
	items := []*widget.FormItem{
		widget.NewFormItem("Text", textEntry),
		widget.NewFormItem("Font size", fontSize),
		widget.NewFormItem("Background", backgroundEntry),
	}
	form = dialog.NewForm("Insert text", "Ok", "Cancel", items,
		func(confirm bool) {
			if confirm {
				fSize, err := strconv.ParseFloat(fontSize.Text, 64)
				if err != nil {
					glog.Errorf("Error parsing the font size given: %q", fontSize.Text)
					vp.gs.status.SetText(fmt.Sprintf("Error parsing the font size given: %q", fontSize.Text))
					return
				}
				vp.FontSize = fSize
				vp.gs.App.Preferences().SetFloat(FontSizePreference, fSize)
				textFilter := filters.NewText(textEntry.Text, center, vp.DrawingColor, vp.BackgroundColor, fSize)
				vp.gs.Filters = append(vp.gs.Filters, textFilter)
				vp.gs.ApplyFilters(true)
				vp.gs.status.SetText("Text drawn, use Control+Z to undo.")
			}
		}, vp.gs.Win)
	form.Resize(fyne.NewSize(500, 300))
	form.Show()
	vp.gs.Win.Canvas().Focus(textEntry)
}

// cropTopLeft will crop the screenshot on this position.
func (vp *ViewPort) cropTopLeft(x, y int) {
	vp.gs.CropRect.Min = vp.gs.CropRect.Min.Add(image.Point{X: x, Y: y})
	vp.gs.ApplyFilters(true)
	vp.viewX, vp.viewY = 0, 0 // Move view to cropped corner.
	glog.V(2).Infof("cropTopLeft: new cropRect is %+v", vp.gs.CropRect)
	vp.postCrop()
}

// cropBottomRight will crop the screenshot on this position.
func (vp *ViewPort) cropBottomRight(x, y int) {
	vp.gs.CropRect.Max = vp.gs.CropRect.Max.Sub(
		image.Point{X: vp.gs.CropRect.Dx() - x, Y: vp.gs.CropRect.Dy() - y})
	vp.gs.ApplyFilters(true)
	vp.viewX, vp.viewY = x-vp.viewW, y-vp.viewH // Move view to cropped corner.
	vp.postCrop()
}

func (vp *ViewPort) cropReset() {
	vp.viewX += vp.gs.CropRect.Min.X
	vp.viewY += vp.gs.CropRect.Min.Y
	vp.gs.CropRect = vp.gs.OriginalScreenshot.Rect
	vp.gs.ApplyFilters(true)
	vp.postCrop()
	vp.gs.status.SetText(fmt.Sprintf("Reset to original screenshot of size %d x %d pixels.",
		vp.gs.CropRect.Dx(), vp.gs.CropRect.Dy()))
}

// postCrop refreshes elements after a change in crop.
func (vp *ViewPort) postCrop() {
	// Full image fits the view port in any of the dimensions, then we center the image.
	if vp.gs.CropRect.Dx() < vp.viewW {
		vp.viewX = -(vp.viewW - vp.gs.CropRect.Dx()) / 2
	}
	if vp.gs.CropRect.Dy() < vp.viewH {
		vp.viewY = -(vp.viewH - vp.gs.CropRect.Dy()) / 2
	}

	vp.updateViewSize()
	vp.renderCache()
	vp.Refresh()
	vp.gs.miniMap.updateViewPortRect()
	vp.gs.miniMap.Refresh()
	vp.gs.status.SetText(fmt.Sprintf("New crop: {%d, %d} - {%d, %d} of original screen, %d x %d pixels.",
		vp.gs.CropRect.Min.X, vp.gs.CropRect.Min.Y, vp.gs.CropRect.Max.X, vp.gs.CropRect.Max.Y,
		vp.gs.CropRect.Dx(), vp.gs.CropRect.Dy()))
}
