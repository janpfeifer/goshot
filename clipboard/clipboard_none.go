//go:build !linux && !windows && !darwin
// +build !linux,!windows,!darwin

package clipboard

// Placeholder implementation that informs about missing capability.

import (
	"errors"
	"image"
)

func CopyImage(img image.Image) error {
	return errors.New("Clipboard image copy not implemented in this platform, sorry.")
}

func CopyText(text string) error {
	return errors.New("Clipboard text copy not implemented in this platform, sorry.")
}
