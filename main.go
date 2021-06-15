package main

import (
	"bytes"
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"github.com/janpfeifer/goshot/clipboard"
	"image/draw"
	"image/png"
	"path"

	//"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"github.com/kbinani/screenshot"
	"image"
	"time"
)

type GoShot struct {
	// Fyne: Application and Window
	App fyne.App
	Win fyne.Window // Main window.

	// Original screenshot information
	OriginalScreenshot *image.RGBA
	ScreenshotTime     time.Time

	// Edited screenshot
	Screenshot *image.RGBA // The edited/composed screenshot
	CropRect   image.Rectangle
	Filters    []ImageFilter // Configured filters: each filter is one edition to the image.

	// UI elements
	zoomEntry, thicknessEntry *widget.Entry
	colorSample               *canvas.Rectangle
	status                    *widget.Label
	viewPort                  *ViewPort
	viewPortScroll            *container.Scroll
	miniMap                   *MiniMap
}

type ImageFilter interface {
	// Apply filter, shifted (dx, dy) pixels -- e.g. if a filter draws a circle on
	// top of the image, it should add (dx, dy) to the circle center.
	Apply(image image.Image) image.Image
}

// ApplyFilters will apply `Filters` to the `CropRect` of the original image
// and regenerate Screenshot.
// If full == true, regenerates full Screenshot. If false, renerates only
// visible area.
func (gs *GoShot) ApplyFilters(full bool) {
	glog.V(2).Infof("ApplyFilters: %d filters", len(gs.Filters))
	filteredImage := image.Image(gs.OriginalScreenshot)
	for _, filter := range gs.Filters {
		filteredImage = filter.Apply(filteredImage)
	}

	if gs.Screenshot == gs.OriginalScreenshot || gs.Screenshot.Rect.Dx() != gs.CropRect.Dx() || gs.Screenshot.Rect.Dy() != gs.CropRect.Dy() {
		// Recreate image buffer.
		crop := image.NewRGBA(image.Rect(0, 0, gs.CropRect.Dx(), gs.CropRect.Dy()))
		gs.Screenshot = crop
		full = true // Regenerate the full buffer.
	}
	if full {
		draw.Src.Draw(gs.Screenshot, gs.Screenshot.Rect, filteredImage, gs.CropRect.Min)
	} else {
		var tgtRect image.Rectangle
		tgtRect.Min = image.Point{X: gs.viewPort.viewX, Y: gs.viewPort.viewY}
		tgtRect.Max = tgtRect.Min.Add(image.Point{X: gs.viewPort.viewW, Y: gs.viewPort.viewH})
		srcPoint := gs.CropRect.Min.Add(tgtRect.Min)
		draw.Src.Draw(gs.Screenshot, tgtRect, filteredImage, srcPoint)
	}

	if gs.viewPort != nil {
		gs.viewPort.renderCache()
		gs.viewPort.Refresh()
	}
	if gs.miniMap != nil {
		gs.miniMap.renderCache()
		gs.miniMap.Refresh()
	}
}

func main() {
	flag.Parse()
	gs := &GoShot{
		App: app.NewWithID("GoShot"),
	}
	if err := gs.MakeScreenshot(); err != nil {
		glog.Fatalf("Failed to capture screenshot: %s", err)
	}
	gs.BuildEditWindow()
	gs.Win.ShowAndRun()
}

func (gs *GoShot) MakeScreenshot() error {
	n := screenshot.NumActiveDisplays()
	if n != 1 {
		glog.Warningf("No support for multiple displays yet (should be relatively easy to add), screenshotting first display.")
	}
	bounds := screenshot.GetDisplayBounds(0)
	var err error
	gs.Screenshot, err = screenshot.CaptureRect(bounds)
	if err != nil {
		return err
	}
	gs.OriginalScreenshot = gs.Screenshot
	gs.ScreenshotTime = time.Now()
	gs.CropRect = gs.Screenshot.Bounds()

	glog.V(2).Infof("Screenshot captured bounds: %+v\n", bounds)
	return nil
	//
	//fileName := fmt.Sprintf("%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
	//file, _ := os.Create(fileName)
	//defer file.Close()
	//png.Encode(file, img)
	//
	//fmt.Printf("#%d : %v \"%s\"\n", i, bounds, fileName)
}

// UndoLastFilter cancels the last filter applied, and regenerates everything.
func (gs *GoShot) UndoLastFilter() {
	if len(gs.Filters) > 0 {
		gs.Filters = gs.Filters[:len(gs.Filters)-1]
		gs.ApplyFilters(true)
	}
}

// DefaultName retuns a default name to the screenshot, based on date/time it was made.
func (gs *GoShot) DefaultName() string {
	return fmt.Sprintf("Screenshot %s",
		gs.ScreenshotTime.Format("2006-01-02 15-04-02"))
}

const DefaultPathPreference = "DefaultPath"

// SaveImage opens a file save dialog box to save the currently edited screenshot.
func (gs *GoShot) SaveImage() {
	glog.V(2).Info("SaveImage")
	var fileSave *dialog.FileDialog
	fileSave = dialog.NewFileSave(
		func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				glog.Errorf("Failed to save image: %s", err)
				gs.status.SetText(fmt.Sprintf("Failed to save image: %s", err))
				return
			}
			if writer == nil {
				gs.status.SetText("Save file cancelled.")
				return
			}
			glog.V(2).Infof("SaveImage(): URI=%s", writer.URI())
			defer writer.Close()

			// Always default to previous path used:
			defaultPath := path.Dir(writer.URI().Path())
			gs.App.Preferences().SetString(DefaultPathPreference, defaultPath)

			var contentBuffer bytes.Buffer
			png.Encode(&contentBuffer, gs.Screenshot)
			content := contentBuffer.Bytes()
			_, err = writer.Write(content)
			if err != nil {
				glog.Errorf("Failed to save image to %q: %s", writer.URI(), fileSave)
				gs.status.SetText(fmt.Sprintf("Failed to save image to %q: %s", writer.URI(), err))
				return
			}
			gs.status.SetText(fmt.Sprintf("Saved image to %q", writer.URI()))
		}, gs.Win)
	fileSave.SetFileName(gs.DefaultName() + ".png")
	if defaultPath := gs.App.Preferences().String(DefaultPathPreference); defaultPath != "" {
		lister, err := storage.ListerForURI(storage.NewFileURI(defaultPath))
		if err == nil {
			fileSave.SetLocation(lister)
		} else {
			glog.Warningf("Cannot create a ListableURI for %q", defaultPath)
		}
	}
	size := gs.Win.Canvas().Size()
	size.Width *= 0.90
	size.Height *= 0.90
	fileSave.Resize(size)
	fileSave.Show()
}

func (gs *GoShot) CopyImageToClipboard() {
	glog.V(2).Info("CopyImageToClipboard")
	err := clipboard.CopyImage(gs.Screenshot)
	if err != nil {
		glog.Errorf("Failed to copy to clipboard: %s", err)
		gs.status.SetText(fmt.Sprintf("Failed to copy to clipboard: %s", err))
	} else {
		gs.status.SetText(fmt.Sprintf("Screenshot copied to clipboard"))
	}
}

func shareWithGoogleDrive() {
	fmt.Println("shareWithGoogleDrive")
}
