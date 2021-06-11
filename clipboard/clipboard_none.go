// +build !linux,!windows

package clipboard

// Placeholder implementation that informs about missing capability.

import "errors"

func CopyImage(content []byte) error {
	return errors.New("Clipboard image copy not implemented in this platform, sorry.")
}
