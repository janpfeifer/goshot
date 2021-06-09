package main

import (
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/janpfeifer/goshot/resources"
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

	// Screenshot information
	Screenshot, OriginalScreenshot *image.RGBA
	ScreenshotTime                 time.Time
	Crop                           image.Rectangle

	// UI elements
	zoomEntry      *widget.Entry
	status         *widget.Label
	viewPort       *ViewPort
	viewPortScroll *container.Scroll
	miniMap        *MiniMap
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
	gs.Crop = gs.Screenshot.Bounds()

	glog.Infof("Bounds: %+v\n", bounds)
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
	menuFile := fyne.NewMenu("File") // fyne.NewMenuItem("Exit", func() { gsApp.Quit() } ),

	menuShare := fyne.NewMenu("Share",
		fyne.NewMenuItem("Copy (clipboard)", func() { copyImageToClipboard() }),
		fyne.NewMenuItem("GoogleDrive", func() { shareWithGoogleDrive() }),
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

	gs.miniMap = NewMiniMap(gs, gs.viewPort)
	toolBar := container.NewVBox(
		gs.miniMap,
		container.NewHBox(
			widget.NewLabel("Crop:"),
			cropTopLeft,
			cropBottomRight,
			cropReset,
		),
		widget.NewButton("Arrow", nil),
		widget.NewButton("Circle", nil),
		widget.NewButton("Text", nil),
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

	gs.Win.SetContent(container.NewBorder(
		nil, statusBar, nil, nil, container.NewMax(split)))
	gs.Win.Resize(fyne.NewSize(800.0, 600.0))
}

func copyImageToClipboard() {
	fmt.Println("copyImageToClipboard")
}

func shareWithGoogleDrive() {
	fmt.Println("shareWithGoogleDrive")
}
