package main

import (
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
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
	Screenshot     *image.RGBA
	ScreenshotTime time.Time
	Crop           image.Rectangle

	// UI elements
	zoomEntry      *widget.Entry
	statusValue    *widget.Label
	viewPort       *ViewPort
	viewPortScroll *container.Scroll
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

	// Side toolbar.
	toolBar := container.NewVBox(
		widget.NewButton("Crop", nil),
		widget.NewButton("Arrow", nil),
		widget.NewButton("Circle", nil),
		widget.NewButton("Text", nil),
	)

	// Image canvas.
	// canvasImg := canvas.NewImageFromImage(gs.Screenshot)
	gs.viewPort = NewViewPort(gs)
	gs.viewPortScroll = container.NewScroll(gs.viewPort) // canvasImg)
	//canvasImg.SetMinSize(fyne.NewSize(100.0, 100.0))

	// Status bar.
	gs.zoomEntry = &widget.Entry{Validator: validation.NewRegexp(`\d`, "Must contain a number")}
	gs.zoomEntry.SetPlaceHolder("0.0")
	gs.zoomEntry.OnChanged = func(str string) {
		glog.V(2).Infof("Zoom level changed to %s", str)
		val, err := strconv.ParseFloat(str, 64)
		if err == nil {
			gs.viewPort.Log2Zoom = val
			gs.viewPort.Refresh()
		}
	}
	gs.statusValue = widget.NewLabel(fmt.Sprintf("Rect: %s", gs.Screenshot.Bounds()))

	statusBar := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(widget.NewLabel("Zoom:"), gs.zoomEntry),
		gs.statusValue,
	)

	// Stitch all together.
	split := container.NewHSplit(
		gs.viewPortScroll,
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
