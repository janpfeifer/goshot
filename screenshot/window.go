package screenshot

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/resources"
	"image/color"
	"strconv"
)

func (gs *GoShot) BuildEditWindow() {
	gs.Win = gs.App.NewWindow(fmt.Sprintf("GoShot: screenshot @ %s", gs.ScreenshotTime.Format("2006-01-02 15:04:05")))
	gs.Win.SetIcon(resources.GoShotIconPng)

	// Build menu.
	menuFile := fyne.NewMenu("File",
		fyne.NewMenuItem(fmt.Sprintf("Save (%s)", SaveShortcutDesc), func() { gs.SaveImage() }),
		fyne.NewMenuItem("Delayed screenshot", func() { gs.DelayedScreenshotForm() }),
	) // Quit is added automatically.

	menuShare := fyne.NewMenu("Share",
		fyne.NewMenuItem(fmt.Sprintf("Copy (%s)", CopyShortcutDesc), func() { gs.CopyImageToClipboard() }),
		fyne.NewMenuItem(fmt.Sprintf("GoogleDrive (%s)", DriveShortcutDesc), func() { gs.ShareWithGoogleDrive() }),
	)
	menuHelp := fyne.NewMenu("Help",
		fyne.NewMenuItem("Shortcuts (ctrl+?)", func() { gs.ShowShortcutsPage() }),
	)
	mainMenu := fyne.NewMainMenu(menuFile, menuShare, menuHelp)
	gs.Win.SetMainMenu(mainMenu)

	// Image canvas.
	gs.viewPort = NewViewPort(gs)

	// Side toolbar.
	cropTopLeft := widget.NewButtonWithIcon("", resources.CropTopLeft,
		func() {
			gs.status.SetText("Click on new top-left corner")
			gs.viewPort.SetOp(CropTopLeft)
		})
	cropBottomRight := widget.NewButtonWithIcon("", resources.CropBottomRight,
		func() {
			gs.status.SetText("Click on new bottom-right corner")
			gs.viewPort.SetOp(CropBottomRight)
		})
	cropReset := widget.NewButtonWithIcon("", resources.Reset, func() {
		gs.viewPort.cropReset()
		gs.viewPort.SetOp(NoOp)
	})

	circleButton := widget.NewButton("Circle (alt+c)", func() { gs.viewPort.SetOp(DrawCircle) })
	circleButton.SetIcon(resources.DrawCircle)

	gs.thicknessEntry = &widget.Entry{Validator: validation.NewRegexp(`\d`, "Must contain a number")}
	gs.thicknessEntry.SetPlaceHolder(fmt.Sprintf("%g", gs.viewPort.Thickness))
	gs.thicknessEntry.OnChanged = func(str string) {
		glog.V(2).Infof("Thickness changed to %s", str)
		val, err := strconv.ParseFloat(str, 64)
		if err == nil {
			gs.viewPort.Thickness = val
			gs.App.Preferences().SetFloat(ThicknessPreference, val)
		}
	}

	gs.colorSample = canvas.NewRectangle(gs.viewPort.DrawingColor)
	size1d := theme.IconInlineSize()
	size := fyne.NewSize(5*size1d, size1d)
	gs.colorSample.SetMinSize(size)
	gs.colorSample.Resize(size)

	gs.miniMap = NewMiniMap(gs, gs.viewPort)
	toolBar := container.NewVBox(
		gs.miniMap,
		container.NewHBox(
			widget.NewLabel("Crop:"),
			cropTopLeft,
			cropBottomRight,
			cropReset,
		),
		widget.NewButtonWithIcon("Arrow (alt+a)", resources.DrawArrow,
			func() { gs.viewPort.SetOp(DrawArrow) }),
		circleButton,
		container.NewHBox(
			widget.NewIcon(resources.Thickness), gs.thicknessEntry,
			widget.NewButtonWithIcon("", resources.ColorWheel, func() { gs.colorPicker() }),
			gs.colorSample,
		),
		widget.NewButtonWithIcon("Text (alt+t)", resources.DrawText,
			func() { gs.viewPort.SetOp(DrawText) }),
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
	gs.Win.Resize(fyne.NewSize(1024.0, 768.0))

	// Register shortcuts.
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: desktop.ControlModifier},
		func(shortcut fyne.Shortcut) {
			glog.Infof("Quit requested by shortcut %s", shortcut.ShortcutName())
			gs.App.Quit()
		})

	gs.RegisterShortcuts()
}

func (gs *GoShot) colorPicker() {
	glog.V(2).Infof("colorPicker():")
	picker := dialog.NewColorPicker(
		"Pick a Color", "Select color for edits",
		func(c color.Color) {
			gs.viewPort.DrawingColor = c
			gs.SetColorPreference(DrawingColorPreference, c)
			gs.colorSample.FillColor = c
			gs.colorSample.Refresh()
		},
		gs.Win)
	picker.Show()
}
