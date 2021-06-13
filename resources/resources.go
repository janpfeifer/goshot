package resources

// This file embeds all the resources used by the program.
//
// colorwheel.svg was copied and modified from wikimedia: "No machine-readable author provided.
// MarianSigler assumed (based on copyright claims)., Public domain, via Wikimedia Commons"

import (
	_ "embed"
	"fyne.io/fyne/v2"
)

//go:embed reset.png
var embedReset []byte
var Reset = fyne.NewStaticResource("reset", embedReset)

//go:embed crop_top_left.png
var embedCropTopLeft []byte
var CropTopLeft = fyne.NewStaticResource("", embedCropTopLeft)

//go:embed crop_bottom_right.png
var embedCropBottomRight []byte
var CropBottomRight = fyne.NewStaticResource("", embedCropBottomRight)

//go:embed draw_circle.png
var embedDrawCircle []byte
var DrawCircle = fyne.NewStaticResource("", embedDrawCircle)

//go:embed draw_arrow.png
var embedDrawArrow []byte
var DrawArrow = fyne.NewStaticResource("", embedDrawArrow)

//go:embed thickness.png
var embedThickness []byte
var Thickness = fyne.NewStaticResource("Thickness", embedThickness)

//go:embed colors.png
var embedColors []byte
var Colors = fyne.NewStaticResource("Colors", embedColors)

//go:embed colorwheel.png
var embedColorWheel []byte
var ColorWheel = fyne.NewStaticResource("ColorWheel", embedColorWheel)
