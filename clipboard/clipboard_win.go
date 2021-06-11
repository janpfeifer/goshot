//+build windows

package clipboard

import "time"

// Windows clipboard manager version: based on https://github.com/atotto/clipboard.

// #include <stdlib.h>
import "C"

import (
	"github.com/golang/glog"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	cfUnicodetext = 13
	gmemMoveable  = 0x0002

	clipboardOpenMaxTime = 1 * time.Second
)

var (
	user32                     = syscall.MustLoadDLL("user32")
	isClipboardFormatAvailable = user32.MustFindProc("IsClipboardFormatAvailable")
	openClipboard              = user32.MustFindProc("OpenClipboard")
	closeClipboard             = user32.MustFindProc("CloseClipboard")
	emptyClipboard             = user32.MustFindProc("EmptyClipboard")
	getClipboardData           = user32.MustFindProc("GetClipboardData")
	setClipboardData           = user32.MustFindProc("SetClipboardData")
	registerClipboardFormat    = user32.MustFindProc("RegisterClipboardFormatA")

	kernel32      = syscall.NewLazyDLL("kernel32")
	globalAlloc   = kernel32.NewProc("GlobalAlloc")
	globalFree    = kernel32.NewProc("GlobalFree")
	globalLock    = kernel32.NewProc("GlobalLock")
	globalUnlock  = kernel32.NewProc("GlobalUnlock")
	rtlMoveMemory = kernel32.NewProc("RtlMoveMemory")
)

func CopyImage(content []byte) error {
	glog.V(2).Infof("copyImageToClipboard(%d bytes)", len(content))
	writeAll(content)
	return nil
}

// waitOpenClipboard opens the clipboard, polling every 3 milliseconds
// and waiting for up to a few seconds to do so (`clipboardOpenMaxTime`).
func waitOpenClipboard() error {
	started := time.Now()
	limit := started.Add(clipboardOpenMaxTime)
	var r uintptr
	var err error
	for time.Now().Before(limit) {
		r, _, err = openClipboard.Call(0)
		if r != 0 {
			return nil
		}
		time.Sleep(3 * time.Millisecond)
	}
	return err
}

func writeAll(content []byte) error {
	// LockOSThread ensure that the whole method will keep executing on the same thread from begin to end (it actually locks the goroutine thread attribution).
	// Otherwise if the goroutine switch thread during execution (which is a common practice), the OpenClipboard and CloseClipboard will happen on two different threads, and it will result in a clipboard deadlock.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := waitOpenClipboard()
	if err != nil {
		return err
	}
	glog.V(2).Infof("writeAll: clipboard opened.")

	r, _, err := emptyClipboard.Call(0)
	if r == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}
	glog.V(2).Infof("writeAll: clipboard emptied.")

	// "If the hMem parameter identifies a memory object, the object must have
	// been allocated using the function with the GMEM_MOVEABLE flag."
	movableDataHandle, _, err := globalAlloc.Call(gmemMoveable, uintptr(len(content)))
	if movableDataHandle == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}
	defer func() {
		if movableDataHandle != 0 {
			globalFree.Call(movableDataHandle)
		}
	}()
	glog.V(2).Infof("writeAll: got moveableDataHandle=%0xd", movableDataHandle)

	lockedData, _, err := globalLock.Call(movableDataHandle)
	if lockedData == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}
	glog.V(2).Infof("writeAll: mapped data handle to %0xd", lockedData)

	//r, _, err = lstrcpy.Call(l, uintptr(unsafe.Pointer(&data[0])))
	r, _, err = rtlMoveMemory.Call(lockedData, uintptr(unsafe.Pointer(&content[0])), uintptr(len(content)))
	if r == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}
	glog.V(2).Infof("writeAll: content copied")

	r, _, err = globalUnlock.Call(movableDataHandle)
	if r == 0 {
		if err.(syscall.Errno) != 0 {
			_, _, _ = closeClipboard.Call()
			return err
		}
	}
	glog.V(2).Infof("writeAll: globalUnlock'ed")

	clipboardFormat := C.CString("image/png")
	//clipboardFormat := C.CString("PNG")
	formatId, _, err := registerClipboardFormat.Call(uintptr(unsafe.Pointer(clipboardFormat)))
	defer C.free(unsafe.Pointer(clipboardFormat))
	glog.V(2).Infof("writeAll: formatId=%d", formatId)

	r, _, err = setClipboardData.Call(uintptr(formatId), movableDataHandle)
	//r, _, err = setClipboardData.Call(C.CF_TEXT, movableDataHandle)
	if r == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}
	glog.V(2).Infof("writeAll: presumably clipboard set ?")

	movableDataHandle = 0 // Ownership transferred, suppress deferred cleanup.
	closed, _, err := closeClipboard.Call()
	if closed == 0 {
		return err
	}
	return nil
}
