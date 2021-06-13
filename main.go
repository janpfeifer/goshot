package main

import (
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/janpfeifer/goshot/clipboard"
	"github.com/janpfeifer/goshot/resources"
	"image/draw"
	"strconv"

	//"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
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
	zoomEntry      *widget.Entry
	status         *widget.Label
	viewPort       *ViewPort
	viewPortScroll *container.Scroll
	miniMap        *MiniMap
}

type ImageFilter interface {
	// Apply filter, shifted (dx, dy) pixels -- e.g. if a filter draws a circle on
	// top of the image, it should add (dx, dy) to the circle center.
	Apply(image image.Image) image.Image
}

// ApplyFilters will apply `Filters` to the `CropRect` of the original image
// and regenerate Screenshot.
func (gs *GoShot) ApplyFilters() {
	glog.V(2).Infof("ApplyFilters: %d filters", len(gs.Filters))
	filteredImage := image.Image(gs.OriginalScreenshot)
	for _, filter := range gs.Filters {
		filteredImage = filter.Apply(filteredImage)
	}
	crop := image.NewRGBA(image.Rect(0, 0, gs.CropRect.Dx(), gs.CropRect.Dy()))
	draw.Src.Draw(crop, crop.Rect, filteredImage, image.Point{X: gs.CropRect.Min.X, Y: gs.CropRect.Min.Y})
	gs.Screenshot = crop
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

func (gs *GoShot) BuildEditWindow() {
	gs.Win = gs.App.NewWindow(fmt.Sprintf("GoShot: Screenshot at %s", gs.ScreenshotTime))

	// Build menu.
	menuFile := fyne.NewMenu("File") // Quit is added automatically.

	menuShare := fyne.NewMenu("Share",
		fyne.NewMenuItem("Copy (ctrl+c)", func() { copyImageToClipboard(gs) }),
		fyne.NewMenuItem("GoogleDrive (ctrl+d)", func() { shareWithGoogleDrive() }),
	)
	mainMenu := fyne.NewMainMenu(menuFile, menuShare)
	gs.Win.SetMainMenu(mainMenu)

	// Image canvas.
	gs.viewPort = NewViewPort(gs)

	// Side toolbar.
	cropTopLeft := widget.NewButton("", func() {
		gs.status.SetText("Click on new top-left corner")
		gs.viewPort.SetOp(CropTopLeft)
	})
	cropTopLeft.SetIcon(resources.CropTopLeft)
	cropBottomRight := widget.NewButton("", func() {
		gs.status.SetText("Click on new bottom-right corner")
		gs.viewPort.SetOp(CropBottomRight)
	})
	cropBottomRight.SetIcon(resources.CropBottomRight)
	cropReset := widget.NewButton("", func() {
		gs.viewPort.cropReset()
		gs.viewPort.SetOp(NoOp)
	})
	cropReset.SetIcon(resources.Reset)

	circleButton := widget.NewButton("Circle (alt+c)", func() { gs.viewPort.SetOp(DrawCircle) })
	circleButton.SetIcon(resources.DrawCircle)

	gs.miniMap = NewMiniMap(gs, gs.viewPort)
	toolBar := container.NewVBox(
		gs.miniMap,
		container.NewHBox(
			widget.NewLabel("Crop:"),
			cropTopLeft,
			cropBottomRight,
			cropReset,
		),
		widget.NewButton("Arrow (alt+a)", nil),
		circleButton,
		widget.NewButton("Text (alt+t)", nil),
	)

	// Status bar with zoom control.
	gs.zoomEntry = &widget.Entry{Validator: validation.NewRegexp(`\d`, "Must contain a number")}
	gs.zoomEntry.SetPlaceHolder("0.0")
	gs.zoomEntry.OnChanged = func(str string) {
		glog.V(2).Infof("Zoom level changed to %s", str)
		val, err := strconv.ParseFloat(str, 64)
		if err == nil {
			gs.viewPort.Log2Zoom = val
			gs.viewPort.updateViewSize()
			gs.viewPort.Refresh()
		}
	}
	zoomReset := widget.NewButton("", func() {
		gs.zoomEntry.SetText("0")
		gs.viewPort.Log2Zoom = 0
		gs.viewPort.updateViewSize()
		gs.viewPort.Refresh()
	})
	zoomReset.SetIcon(resources.Reset)
	gs.status = widget.NewLabel(fmt.Sprintf("Image size: %s", gs.Screenshot.Bounds()))
	statusBar := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(widget.NewLabel("Zoom:"), gs.zoomEntry, zoomReset),
		gs.status,
	)

	// Stitch all together.
	split := container.NewHSplit(
		gs.viewPort,
		toolBar,
	)
	split.Offset = 0.8

	topLevel := container.NewBorder(
		nil, statusBar, nil, nil, container.NewMax(split))
	gs.Win.SetContent(topLevel)
	gs.Win.Resize(fyne.NewSize(800.0, 600.0))

	// Register shortcuts.
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{fyne.KeyQ, desktop.ControlModifier},
		func(shortcut fyne.Shortcut) {
			glog.Infof("Quit requested by shortcut %s", shortcut.ShortcutName())
			gs.App.Quit()
		})

	gs.Win.Canvas().AddShortcut(
		&fyne.ShortcutCopy{},
		func(_ fyne.Shortcut) { copyImageToClipboard(gs) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{fyne.KeyC, desktop.AltModifier},
		func(shortcut fyne.Shortcut) {
			printShortcut(shortcut)
			gs.viewPort.SetOp(DrawCircle)
		})
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{fyne.KeyT, desktop.AltModifier}, printShortcut)
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{fyne.KeyA, desktop.AltModifier}, printShortcut)

	gs.Win.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			gs.viewPort.SetOp(NoOp)
		} else {
			glog.V(2).Infof("KeyTyped: %+v", ev)
		}
	})
}

func printShortcut(shortcut fyne.Shortcut) {
	glog.Infof("Shortcut: %s", shortcut.ShortcutName())
}

func copyImageToClipboard(gs *GoShot) {
	glog.V(2).Info("copyImageToClipboard")
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
