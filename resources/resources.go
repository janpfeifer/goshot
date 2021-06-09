package resources

// This file embeds all the resources used by the program.

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