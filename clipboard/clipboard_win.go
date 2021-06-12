//+build windows

package clipboard

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/lxn/win"
	"image"
	"image/png"
	"strings"
	"syscall"
	"time"
)

// Windows clipboard manager version: based on https://github.com/atotto/clipboard.

// #include <stdlib.h>
// #include <string.h>
import "C"

import (
	"github.com/golang/glog"
	"runtime"
	"unsafe"
)

var (
	user32                      = syscall.MustLoadDLL("user32.dll")
	procRegisterClipboardFormat = user32.MustFindProc("RegisterClipboardFormatA")
)

const (
	LCS_WINDOWS_COLOR_SPACE = 0x57696E20

	clipboardOpenMaxTime = 1 * time.Second
)

func CopyImage(img image.Image) error {
	glog.V(2).Infof("CopyImage(bounds=%+v)", img.Bounds())

	// CF_DIBV5 version
	hBitmap, bitmapHeader, bitmapBits, err := hBitmapV5FromImage(img)
	if err != nil {
		return err
	}
	glog.V(2).Infof("sizeof(BITMAPV5HEADER)=%d", unsafe.Sizeof(*bitmapHeader))
	glog.V(2).Infof("sizeof(BITMAPINFOHEADER)=%d", unsafe.Sizeof(bitmapHeader.BITMAPINFOHEADER))
	dibV5Data, err := bitmapToGlobalAlloc(&bitmapHeader.BITMAPINFOHEADER, bitmapBits)
	win.GlobalFree(win.HGLOBAL(hBitmap))
	if err != nil {
		return err
	}
	defer func() {
		if dibV5Data != 0 {
			win.GlobalFree(dibV5Data)
		}
	}()

	// CF_DIB version: it's only needed because Chromium is broken, see
	// discussion in
	// https://github.com/tannerhelland/PhotoDemon/issues/343

	// "PNG" format.
	var pngContentBuffer bytes.Buffer
	_ = png.Encode(&pngContentBuffer, img)
	pngContent := pngContentBuffer.Bytes()
	pngData, err := bytesToGlobalAlloc(&pngContent[0], len(pngContent))
	if err != nil {
		return err
	}
	defer func() {
		if pngData != 0 {
			win.GlobalFree(pngData)
		}
	}()

	// "HTML Format"
	html := imageToHMLEncode(img)
	htmlCStr := C.CString(html)
	htmlData, err := bytesToGlobalAlloc((*byte)(unsafe.Pointer(htmlCStr)), int(C.strlen(htmlCStr)))
	C.free(unsafe.Pointer(htmlCStr))
	if err != nil {
		return err
	}

	err = safeSetClipboardData([]formatAndData{
		//{ Format: win.CF_DIB, Data: win.HANDLE(dibData) },
		{Format: win.CF_DIBV5, Data: win.HANDLE(dibV5Data)},
		{RegisteredFormat: "PNG", Data: win.HANDLE(pngData)},
		{RegisteredFormat: "HTML Format", Data: win.HANDLE(htmlData)},
	})
	dibV5Data = 0
	pngData = 0
	return err
}

type formatAndData struct {
	Format           uint32 // Ignored if registeredFormat is given.
	RegisteredFormat string
	Data             win.HANDLE
}

// safeSetClipboardData is a wrapper around all the clipboard "bureaucracy".
// See formats and their expected contents in
// https://docs.microsoft.com/en-us/windows/win32/dataxchg/standard-clipboard-formats.
//
// No support for registered formats yet -- should be simple to add.
func safeSetClipboardData(formats []formatAndData) error {
	glog.V(2).Infof("safeSetClipboardData()")

	// LockOSThread ensure that the whole method will keep executing on the same thread from begin to end (it actually locks the goroutine thread attribution).
	// Otherwise if the goroutine switch thread during execution (which is a common practice), the OpenClipboard and CloseClipboard will happen on two different threads, and it will result in a clipboard deadlock.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := waitOpenClipboard()
	if err != nil {
		return err
	}
	glog.V(2).Infof("safeSetClipboardData: clipboard opened.")
	defer win.CloseClipboard()

	if !win.EmptyClipboard() {
		return errors.New("failed win.EmptyClipboard()")
	}
	glog.V(2).Infof("safeSetClipboardData: clipboard emptied.")

	for _, fd := range formats {
		var res win.HANDLE
		glog.V(2).Infof("- setting %+v", fd)
		if fd.RegisteredFormat == "" {
			res = win.SetClipboardData(fd.Format, fd.Data)
		} else {
			format := registerClipboardFormat(fd.RegisteredFormat)
			res = win.SetClipboardData(format, fd.Data)
		}
		if res == 0 {
			// Free resource, since setting of clipboard failed.
			win.GlobalFree(win.HGLOBAL(fd.Data))
		}
	}
	return nil
}

// Converts bitmap to a contiguous GMEM_MOVEABLE global alloc to be used the clipboard.
// It should work with BITMAPINFOHEADER, BITMAPV4HEADER and BITMAPV5HEADER
// if the biSize field is correctly set.
//
// Palette color map not supported.
func bitmapToGlobalAlloc(bitmapHeader *win.BITMAPINFOHEADER, bitmapBits unsafe.Pointer) (win.HGLOBAL, error) {
	glog.V(2).Infof("bitmapToGlobalAlloc: header size=%d, image size=%d", bitmapHeader.BiSize, bitmapHeader.BiSizeImage)
	// "If the hMem parameter identifies a memory object, the object must have
	// been allocated using the function with the GMEM_MOVEABLE flag."
	movableDataHandle := win.GlobalAlloc(win.GMEM_MOVEABLE, uintptr(bitmapHeader.BiSize+bitmapHeader.BiSizeImage))
	if movableDataHandle == 0 {
		return 0, errors.New("call to GlobalAlloc failed")
	}

	lockedData := win.GlobalLock(movableDataHandle)
	if lockedData == nil {
		return 0, errors.New("call to GlobalLock failed")
	}
	win.MoveMemory(lockedData, unsafe.Pointer(bitmapHeader), uintptr(bitmapHeader.BiSize))
	win.MoveMemory(unsafe.Pointer(uintptr(lockedData)+uintptr(bitmapHeader.BiSize)), bitmapBits, uintptr(bitmapHeader.BiSizeImage))
	win.GlobalUnlock(movableDataHandle)

	return movableDataHandle, nil
}

// bytesToGlobalAlloc converts byte data to a GMEM_MOVEABLE global alloc to be used the clipboard.
func bytesToGlobalAlloc(data *byte, length int) (win.HGLOBAL, error) {
	movableDataHandle := win.GlobalAlloc(win.GMEM_MOVEABLE, uintptr(length))
	if movableDataHandle == 0 {
		return 0, errors.New("call to GlobalAlloc failed")
	}
	lockedData := win.GlobalLock(movableDataHandle)
	if lockedData == nil {
		return 0, errors.New("call to GlobalLock failed")
	}
	win.MoveMemory(lockedData, unsafe.Pointer(data), uintptr(length))
	win.GlobalUnlock(movableDataHandle)
	return movableDataHandle, nil
}

// Creates a hBitmap V5 (DIBV5) bitmap.
func hBitmapV5FromImage(im image.Image) (hBitmap win.HBITMAP, bitmapHeader *win.BITMAPV5HEADER, bitmapBits unsafe.Pointer, err error) {
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

	// Create the DIB section with an alpha channel.
	hBitmap = win.CreateDIBSection(hdc, &bi.BITMAPINFOHEADER, win.DIB_RGB_COLORS, &bitmapBits, 0, 0)
	switch hBitmap {
	case 0, win.ERROR_INVALID_PARAMETER:
		return 0, nil, nil, errors.New("CreateDIBSection failed")
	}
	glog.V(2).Infof("header=%+v", bi)
	glog.V(2).Infof("bitmapBits=%v", bitmapBits)

	// Fill the image
	bitmapArray := (*[1 << 30]byte)(unsafe.Pointer(bitmapBits))
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

	return hBitmap, bi, bitmapBits, nil
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

func registerClipboardFormat(format string) uint32 {
	p := C.CString(format)
	r, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(p)))
	C.free(unsafe.Pointer(p))
	return uint32(r)
}

func imageToHMLEncode(img image.Image) string {
	var pngContentBuffer bytes.Buffer
	_ = png.Encode(&pngContentBuffer, img)
	convertWinLF := func(str string) string {
		return strings.ReplaceAll(str, "\n", "\r\n")
	}
	headerFn := func(htmlStart, htmlEnd, fragStart, fragEnd int) string {
		return convertWinLF(fmt.Sprintf(
			"Version 0.9\n"+
				"StartHTML:%010d\n"+
				"EndHTML:%010d\n"+
				"StartFragment:%010d\n"+
				"EndFragment:%010d\n"+
				"SourceURL:about:blank\n",
			htmlStart, htmlEnd, fragStart, fragEnd))
	}
	preamble := convertWinLF("<html>\n<body>\n<!--StartFragment-->")
	htmlStart := len(headerFn(0, 0, 0, 0))
	fragmentStart := htmlStart + len(preamble)
	pngBase64 := base64.StdEncoding.EncodeToString(pngContentBuffer.Bytes())
	imgTag := fmt.Sprintf("<img src=\"data:image/png;base64,%s\"/>", pngBase64)
	fragmentEnd := fragmentStart + len(imgTag)
	suffix := convertWinLF(fmt.Sprintf("<!--EndFragment-->\n</body>\n</html>"))
	htmlEnd := fragmentEnd + len(suffix)

	return strings.Join(
		[]string{
			headerFn(htmlStart, htmlEnd, fragmentStart, fragmentEnd),
			preamble,
			imgTag,
			suffix,
		}, "")
}
