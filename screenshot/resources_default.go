//go:build !darwin
// +build !darwin

package screenshot

// This file has the default resources for things like menu entries and related stuff, that can be specialized
// for different platforms.
const (
	SaveShortcutDesc  = "ctrl+s"
	CopyShortcutDesc  = "ctrl+c"
	DriveShortcutDesc = "ctrl+g"
)
