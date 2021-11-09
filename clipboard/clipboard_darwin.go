//go:build darwin
// +build darwin

package clipboard

import (
	"bytes"
	"image"
	"image/png"

	"golang.design/x/clipboard"
)

func CopyImage(img image.Image) error {
	// create buffer
	buff := new(bytes.Buffer)

	// encode image to buffer
	err := png.Encode(buff, img)
	if err != nil {
		return err
	}

	clipboard.Write(clipboard.FmtImage, buff.Bytes())
	return nil
}

func CopyText(text string) error {
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}
