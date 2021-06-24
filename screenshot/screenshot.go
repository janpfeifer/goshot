// Package screenshot implements the screenshot edit window.
//
// It's the main part of the application: it may be run after a
// fork(), if the main program was started as a system tray app.
package screenshot

import (
	"bytes"
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/clipboard"
	"github.com/janpfeifer/goshot/googledrive"
	"github.com/kbinani/screenshot"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"path"
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
	shortcutsDialog           dialog.Dialog

	// GoogleDrive manager
	gDrive          *googledrive.Manager
	gDriveNumShared int
}

type ImageFilter interface {
	// Apply filter, shifted (dx, dy) pixels -- e.g. if a filter draws a circle on
	// top of the image, it should add (dx, dy) to the circle center.
	Apply(image image.Image) image.Image
}

// ApplyFilters will apply `Filters` to the `CropRect` of the original image
// and regenerate Screenshot.
// If full == true, regenerates full Screenshot. If false, regenerates only
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

func Run() {
	gs := &GoShot{
		App: app.NewWithID("GoShot"),
	}
	if err := gs.MakeScreenshot(); err != nil {
		glog.Fatalf("Failed to capture screenshot: %s", err)
	}
	gs.BuildEditWindow()
	gs.Win.ShowAndRun()
	gs.miniMap.updateViewPortRect()
	gs.miniMap.Refresh()
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

// DefaultName returns a default name to the screenshot, based on date/time it was made.
func (gs *GoShot) DefaultName() string {
	return fmt.Sprintf("Screenshot %s",
		gs.ScreenshotTime.Format("2006-01-02 15-04-02"))
}

// GetColorPreference returns the color set for the given key if it has been set.
// Otherwise it returns `defaultColor`.
func (gs *GoShot) GetColorPreference(key string, defaultColor color.RGBA) color.RGBA {
	isSet := gs.App.Preferences().Bool(key)
	if !isSet {
		return defaultColor
	}
	r := gs.App.Preferences().Int(key + "_R")
	g := gs.App.Preferences().Int(key + "_G")
	b := gs.App.Preferences().Int(key + "_B")
	a := gs.App.Preferences().Int(key + "_A")
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

// SetColorPreference sets the given color in the given preferences key.
func (gs *GoShot) SetColorPreference(key string, c color.Color) {
	r, g, b, a := c.RGBA()
	gs.App.Preferences().SetInt(key+"_R", int(r))
	gs.App.Preferences().SetInt(key+"_G", int(g))
	gs.App.Preferences().SetInt(key+"_B", int(b))
	gs.App.Preferences().SetInt(key+"_A", int(a))
	gs.App.Preferences().SetBool(key, true)
}

const DefaultPathPreference = "DefaultPath"

// SaveImage opens a file save dialog box to save the currently edited screenshot.
func (gs *GoShot) SaveImage() {
	glog.V(2).Info("GoShot.SaveImage")
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
			defer func() { _ = writer.Close() }()

			// Always default to previous path used:
			defaultPath := path.Dir(writer.URI().Path())
			gs.App.Preferences().SetString(DefaultPathPreference, defaultPath)

			var contentBuffer bytes.Buffer
			_ = png.Encode(&contentBuffer, gs.Screenshot)
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
	glog.V(2).Info("GoShot.CopyImageToClipboard")
	err := clipboard.CopyImage(gs.Screenshot)
	if err != nil {
		glog.Errorf("Failed to copy to clipboard: %s", err)
		gs.status.SetText(fmt.Sprintf("Failed to copy to clipboard: %s", err))
	} else {
		gs.status.SetText(fmt.Sprintf("Screenshot copied to clipboard"))
	}
}

const (
	GoogleDriveTokenPreference = "google_drive_token"
)

var (
	GoogleDrivePath = []string{"GoShot"}
)

func (gs *GoShot) ShareWithGoogleDrive() {
	glog.V(2).Infof("GoShot.ShareWithGoogleDrive")
	ctx := context.Background()

	gs.status.SetText("Connecting to GoogleDrive ...")
	fileName := gs.DefaultName()
	gs.gDriveNumShared++
	if gs.gDriveNumShared > 1 {
		// In case the screenshot is shared multiple times (after different editions), we want
		// a different name for each.
		fileName = fmt.Sprintf("%s_%d", fileName, gs.gDriveNumShared)
	}

	go func() {
		if gs.gDrive == nil {
			// Create googledrive.Manager.
			token := gs.App.Preferences().String(GoogleDriveTokenPreference)
			var err error
			gs.gDrive, err = googledrive.New(ctx, GoogleDrivePath, token,
				func(token string) { gs.App.Preferences().SetString(GoogleDriveTokenPreference, token) },
				gs.askForGoogleDriveAuthorization)
			if err != nil {
				glog.Errorf("Failed to connect to Google Drive: %s", err)
				gs.status.SetText(fmt.Sprintf("GoogleDrive failed: %v", err))
				return
			}
		}

		// Sharing the image must happen in a separate goroutine because the UI must
		// remain interactive, also in order to capture the authorization input
		// from the user.
		url, err := gs.gDrive.ShareImage(ctx, fileName, gs.Screenshot)
		if err != nil {
			glog.Errorf("Failed to share image in Google Drive: %s", err)
			gs.status.SetText(fmt.Sprintf("GoogleDrive failed: %v", err))
			return
		}
		glog.Infof("GoogleDrive's shared URL:\t%s", url)
		err = clipboard.CopyText(url)
		if err == nil {
			gs.status.SetText("Image shared in GoogleDrive, URL copied to clipboard.")
		} else {
			gs.status.SetText("Image shared in GoogleDrive, but failed to copy to clipboard, see URL and error in the logs.")
			glog.Errorf("Failed to copy URL to clipboard: %v", err)
		}
	}()
}

func (gs *GoShot) askForGoogleDriveAuthorization() string {
	replyChan := make(chan string, 1)

	// Create dialog to get the authorization from the user.
	textEntry := widget.NewEntry()
	textEntry.Resize(fyne.NewSize(400, 40))
	items := []*widget.FormItem{
		widget.NewFormItem("Authorization", textEntry),
		widget.NewFormItem("", widget.NewLabel("Paste below the authorization given by GoogleDrive from the browser")),
	}
	form := dialog.NewForm("Google Drive Authorization", "Ok", "Cancel", items,
		func(confirm bool) {
			if confirm {
				replyChan <- textEntry.Text
			} else {
				replyChan <- ""
			}
		}, gs.Win)
	form.Resize(fyne.NewSize(500, 300))
	form.Show()
	gs.Win.Canvas().Focus(textEntry)

	return <-replyChan
}

// RegisterShortcuts adds all the shortcuts and keys GoShot
// listens to.
// When updating here, please update also the `gs.ShowShortcutsPage()`
// method to reflect the changes.
func (gs *GoShot) RegisterShortcuts() {
	gs.Win.Canvas().AddShortcut(
		&fyne.ShortcutCopy{},
		func(_ fyne.Shortcut) { gs.CopyImageToClipboard() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyJ, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(CropTopLeft) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyK, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(CropBottomRight) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(DrawCircle) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyT, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(DrawText) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(DrawArrow) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.UndoLastFilter() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.SaveImage() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyG, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.ShareWithGoogleDrive() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySlash, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.ShowShortcutsPage() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySlash, Modifier: desktop.ControlModifier | desktop.ShiftModifier},
		func(_ fyne.Shortcut) { gs.ShowShortcutsPage() })

	gs.Win.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			if gs.viewPort.currentOperation != NoOp {
				gs.viewPort.SetOp(NoOp)
				gs.status.SetText("Operation cancelled.")
			}
			if gs.shortcutsDialog != nil {
				gs.shortcutsDialog.Hide()
			}
		} else {
			glog.V(2).Infof("KeyTyped: %+v", ev)
		}
	})
}

func (gs *GoShot) ShowShortcutsPage() {
	if gs.shortcutsDialog == nil {
		titleFn := func(title string) (l *widget.Label) {
			l = widget.NewLabel(title)
			l.TextStyle.Bold = true
			return l
		}
		descFn := func(desc string) (l *widget.Label) {
			l = widget.NewLabel(desc)
			l.Alignment = fyne.TextAlignCenter
			return l
		}
		shortcutFn := func(shortcut string) (l *widget.Label) {
			l = widget.NewLabel(shortcut)
			l.TextStyle.Italic = true
			return l
		}
		gs.shortcutsDialog = dialog.NewCustom("GoShot Shortcuts", "Ok",
			container.NewVScroll(container.NewVBox(
				titleFn("Image Manipulation"),
				container.NewGridWithColumns(2,
					descFn("Crop Top-Left"), shortcutFn("Alt+J"),
					descFn("Crop Bottom-Right"), shortcutFn("Alt+K"),
					descFn("Draw Circle"), shortcutFn("Alt+C"),
					descFn("Draw Arrow"), shortcutFn("Alt+A"),
					descFn("Draw Text"), shortcutFn("Alt+T"),
					descFn("Cancel Operation"), shortcutFn("Esc"),
					descFn("Undo Last Drawing"), shortcutFn("Control+Z"),
				),
				titleFn("Sharing Image"),
				container.NewGridWithColumns(2,
					descFn("Copy Image To Clipboard"), shortcutFn("Control+C"),
					descFn("Save Image"), shortcutFn("Control+S"),
					descFn("Google Drive & Copy URL"), shortcutFn("Control+G"),
				),
				titleFn("Other"),
				container.NewGridWithColumns(2,
					descFn("Shortcut page"), shortcutFn("Control+?"),
					descFn("Quit"), shortcutFn("Control+Q"),
				),
			)), gs.Win)
	}
	size := gs.Win.Canvas().Size()
	size.Width *= 0.90
	size.Height *= 0.90
	gs.shortcutsDialog.Resize(size)
	gs.shortcutsDialog.Show()
}
