// +build !linux,!windows

package clipboard

// Placeholder implementation that informs about missing capability.

import "errors"

func CopyImage(img image.Image) error {
	return errors.New("Clipboard image copy not implemented in this platform, sorry.")
}
