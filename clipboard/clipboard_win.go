//+build windows

package clipboard

import (
	"errors"
	"github.com/lxn/win"
	"image"
	"time"
)

// Windows clipboard manager version: based on https://github.com/atotto/clipboard.

// #include <stdlib.h>
import "C"

import (
	"github.com/golang/glog"
	"runtime"
	"unsafe"
)

const (
	LCS_WINDOWS_COLOR_SPACE = 0x57696E20

	clipboardOpenMaxTime = 1 * time.Second
)

func CopyImage(img image.Image) error {
	glog.V(2).Infof("CopyImage(bounds=%+v)", img.Bounds())
	hBitmap, err := hBitmapFromImage(img)
	if err != nil {
		return err
	}
	writeHBitmap(hBitmap)
	return nil
}

// waitOpenClipboard opens the clipboard, polling every 3 milliseconds
// and waiting for up to a few seconds to do so (`clipboardOpenMaxTime`).
func waitOpenClipboard() error {
	started := time.Now()
	limit := started.Add(clipboardOpenMaxTime)
	for time.Now().Before(limit) {
		if win.OpenClipboard(0) {
			return nil
		}
		time.Sleep(3 * time.Millisecond)
	}
	return errors.New("failed win.OpenClipboard()")
}

func writeHBitmap(hBitmap win.HBITMAP) error {
	glog.V(2).Infof("writeHBitmap()")

	// LockOSThread ensure that the whole method will keep executing on the same thread from begin to end (it actually locks the goroutine thread attribution).
	// Otherwise if the goroutine switch thread during execution (which is a common practice), the OpenClipboard and CloseClipboard will happen on two different threads, and it will result in a clipboard deadlock.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := waitOpenClipboard()
	if err != nil {
		return err
	}
	glog.V(2).Infof("writeHBitmap: clipboard opened.")
	defer win.CloseClipboard()

	if !win.EmptyClipboard() {
		return errors.New("failed win.EmptyClipboard()")
	}
	glog.V(2).Infof("writeHBitmap: clipboard emptied.")
	win.SetClipboardData(win.CF_BITMAP, win.HANDLE(hBitmap))
	return nil
}

func hBitmapFromImage(im image.Image) (win.HBITMAP, error) {
	bi := &win.BITMAPV5HEADER{
		BITMAPV4HEADER: win.BITMAPV4HEADER{
			BITMAPINFOHEADER: win.BITMAPINFOHEADER{
				BiWidth:         int32(im.Bounds().Dx()),
				BiHeight:        -int32(im.Bounds().Dy()), // Negative values means image is top-down (y=0 -> top pixels)
				BiPlanes:        1,
				BiBitCount:      32,
				BiCompression:   win.BI_BITFIELDS,
				BiSizeImage:     uint32(im.Bounds().Dx() * im.Bounds().Dy() * 4), // Size in bytes.
				BiXPelsPerMeter: 2834,
				BiYPelsPerMeter: 2834,
			},

			// The following mask specification specifies a supported 32 BPP
			// alpha format for Windows XP.
			BV4RedMask:   0x00FF0000,
			BV4GreenMask: 0x0000FF00,
			BV4BlueMask:  0x000000FF,
			BV4AlphaMask: 0xFF000000,

			BV4CSType: LCS_WINDOWS_COLOR_SPACE,
		},
	}
	bi.BiSize = uint32(unsafe.Sizeof(*bi)) // This size tells that this is a BITMAPV5HEADER.

	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	var lpBits unsafe.Pointer

	// Create the DIB section with an alpha channel.
	hBitmap := win.CreateDIBSection(hdc, &bi.BITMAPINFOHEADER, win.DIB_RGB_COLORS, &lpBits, 0, 0)
	switch hBitmap {
	case 0, win.ERROR_INVALID_PARAMETER:
		return 0, errors.New("CreateDIBSection failed")
	}
	glog.V(2).Infof("header=%+v", &bi)
	// Fill the image
	bitmapArray := (*[1 << 30]byte)(unsafe.Pointer(lpBits))
	i := 0
	for y := im.Bounds().Min.Y; y != im.Bounds().Max.Y; y++ {
		for x := im.Bounds().Min.X; x != im.Bounds().Max.X; x++ {
			r, g, b, a := im.At(x, y).RGBA()
			bitmapArray[i+3] = byte(a >> 8)
			bitmapArray[i+2] = byte(r >> 8)
			bitmapArray[i+1] = byte(g >> 8)
			bitmapArray[i+0] = byte(b >> 8)
			i += 4
		}
	}

	return hBitmap, nil
}
