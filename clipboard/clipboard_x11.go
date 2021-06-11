//+build linux

package clipboard

import (
	"errors"
	"github.com/golang/glog"
	"sync"
	"unsafe"
)

// Clipboard handling in Linux/X11.

/*
#cgo LDFLAGS: -lX11 -lXmu
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xmu/Atoms.h>

Window macro_DefaultRootWindow(Display *dpy) { return DefaultRootWindow(dpy); }
Atom macro_XA_CLIPBOARD(Display *dpy) { return XA_CLIPBOARD(dpy); }
long macro_XMaxRequestSize(Display *dpy) { return XMaxRequestSize(dpy); }
long macro_XExtendedMaxRequestSize(Display *dpy) { return XExtendedMaxRequestSize(dpy); }
*/
import "C"

var clipboardOnce sync.Once

const ImageTarget = "image/png"

var (
	failure error

	// X11 globals
	display                              *C.Display
	window                               C.Window
	atomClipboardSelection               C.Atom
	atomPNGTarget, atomIncr, atomTargets C.Atom
	targetPartSize                       int

	// clipboard global properties
	hasClipboardOwnership bool
	currentContent        []byte
)

func CopyImage(content []byte) error {
	glog.V(2).Infof("copyImageToClipboard(%d bytes)", len(content))
	clipboardOnce.Do(func() { initX11() })
	if failure != nil {
		glog.Errorf("copyImageToClipboard: %s", failure)
		return failure
	}

	C.XSetSelectionOwner(display, atomClipboardSelection, window, C.CurrentTime)
	currentContent = content
	hasClipboardOwnership = true
	return nil
}

// initX11 will start a hidden window to receive selection (clipboard)
// events and act on them. Should be called only once.
func initX11() {
	glog.V(2).Infof("initX11()")
	display = C.XOpenDisplay(nil)
	if display == nil {
		failure = errors.New("cannot open X11 display for clipboard")
		return
	}
	atomClipboardSelection = C.macro_XA_CLIPBOARD(display)

	atomPNGTarget = getAtomFromName(ImageTarget)
	atomIncr = getAtomFromName("INCR")
	atomTargets = getAtomFromName("TARGETS")
	glog.V(2).Infof("- Atom for \"image/png\" target=%+v", atomPNGTarget)

	xExtendedMaxRequestSize := C.macro_XExtendedMaxRequestSize(display)
	xMaxRequestSize := C.macro_XMaxRequestSize(display)
	glog.V(2).Infof("- XMaxRequestSize=%d, XExtendedMaxRequestSize=%d", xMaxRequestSize, xExtendedMaxRequestSize)
	if xExtendedMaxRequestSize != 0 {
		targetPartSize = int(xExtendedMaxRequestSize) / 4
	} else {
		targetPartSize = int(xMaxRequestSize) / 4
	}
	glog.V(2).Infof("- targetPartSize=%d", targetPartSize)

	window = C.XCreateSimpleWindow(display, C.macro_DefaultRootWindow(display),
		/* x, y, width, height */ 0, 0, 1, 1,
		/* border_width, border, background */ 0, 0, 0)
	C.XSelectInput(display, window, C.PropertyChangeMask) // Listen to atomClipboardSelection/clipboard events
	go x11EventLoop()
}

// x11EventLoop endlessly reads new X11 events and awaiting for selection(clipboard) ones.
// "client window" refers to an external window that wants to paste the clipboard content
// offered here.
func x11EventLoop() {
	for {
		var xev = &C.XEvent{}
		glog.V(2).Info("x11EventLoop: XNextEvent")
		C.XNextEvent(display, xev)
		xevType := *(*XEventType)(unsafe.Pointer(xev))
		glog.V(2).Infof("x11EventLoop: event type=%s (%d)", eventTypesNames[xevType], xevType)
		switch xevType {
		case SelectionRequestEventType:
			if hasClipboardOwnership {
				handleSelectionRequest(xev)
			}

		case PropertyNotifyEventType:
			handleSelectionRequest(xev)

		case SelectionClearEventType:
			glog.V(2).Infof("We lost clipboard ownership")
			hasClipboardOwnership = false // No longer owner, but has to continue serving ongoing transfer.
			currentContent = nil

		default:
			glog.Infof("Unhandled event type %d")
		}
	}
}

// handleSelectionRequest is called when another window ("client window") requested to paste our
// clipboard content.
func handleSelectionRequest(xev *C.XEvent) {
	xevType := *(*XEventType)(unsafe.Pointer(xev))
	var r *requestHandler
	finished := false

	switch xevType {
	case SelectionRequestEventType:
		selEv := (*C.XSelectionRequestEvent)(unsafe.Pointer(xev))
		glog.V(2).Infof("handleSelectionRequest(from=%q)", getWindowName(selEv.requestor))

		var found bool
		r, found = liveRequests[selEv.requestor]
		if !found {
			r = &requestHandler{
				win:     selEv.requestor,
				content: currentContent,
			}
			liveRequests[r.win] = r
		}
		switch r.state {
		case Initial:
			glog.V(2).Infof("- selection: %q", getNameFromAtom(selEv.selection))
			glog.V(2).Infof("- target: %q", getNameFromAtom(selEv.target))
			glog.V(2).Infof("- property: %q", getNameFromAtom(selEv.property))
			r.position = 0
			r.selection = selEv.selection
			r.target = selEv.target
			r.property = selEv.property
			r.time = selEv.time
			if selEv.selection != atomClipboardSelection {
				glog.Warningf("- Unknown selection request for %q: we only use the clipboard selection.",
					getNameFromAtom(selEv.selection))
				delete(liveRequests, r.win)
				return // No response.
			}

			if selEv.target == atomTargets {
				finished = r.ReportTargets()
			} else if selEv.target == atomPNGTarget {
				finished = r.SendContent()
			} else {
				glog.Warningf("- Unknown clipboard request of type %q, only %q is supported.",
					getNameFromAtom(selEv.target), getNameFromAtom(atomPNGTarget))
				delete(liveRequests, r.win)
				return // No response.
			}
		}
		var response C.XEvent
		*(*XEventType)(unsafe.Pointer(&response)) = SelectionNotifyEventType
		respNotify := (*C.XSelectionEvent)(unsafe.Pointer(&response))
		respNotify.display = display
		respNotify.requestor = r.win
		respNotify.selection = r.selection
		respNotify.target = r.target
		respNotify.property = r.property
		respNotify.time = r.time
		C.XSendEvent(display, selEv.requestor, 0, 0, &response)
		C.XFlush(display)

	case PropertyNotifyEventType:
		propEv := (*C.XPropertyEvent)(unsafe.Pointer(xev))
		var found bool
		r, found = liveRequests[propEv.window]
		if !found {
			glog.Warningf("Ignoring un-tracked window %q property change.",
				getWindowName(propEv.window))
			C.XSelectInput(display, propEv.window, 0)
			return
		}
		if propEv.state != C.PropertyDelete {
			glog.Warningf("Ignoring property state %d, we only care about delete", propEv.state)
			return
		}
		finished = r.SendContentPart()
	}

	if finished {
		delete(liveRequests, r.win)
	}
}

var liveRequests = make(map[C.Window]*requestHandler)

type requestHandler struct {
	win                         C.Window
	content                     []byte // Copy reference since, content may change while still serving previous one.
	position                    int
	state                       requestState
	selection, target, property C.Atom
	time                        C.Time
}

// ReportTargets reports available targets and whether request was finished.
func (r *requestHandler) ReportTargets() bool {
	data := [2]C.Atom{atomTargets, atomPNGTarget}
	C.XChangeProperty(display, r.win, r.property, C.XA_ATOM,
		/* format: 32 bits */ 32 /* mode */, C.PropModeReplace,
		(*C.uchar)(unsafe.Pointer(&data)), C.int(len(data)))

	return true
}

func (r *requestHandler) SendContent() bool {
	if len(r.content) <= targetPartSize {
		glog.V(2).Infof("SendContent(): %d bytes at once.", len(r.content))
		// Send all at once.
		C.XChangeProperty(display, r.win, r.property, r.target,
			/* format: byte */ 8 /* mode */, C.PropModeReplace,
			(*C.uchar)(unsafe.Pointer(&r.content[0])), C.int(len(r.content)))
		return true
	}

	// Send in parts.
	glog.V(2).Infof("SendContent(): send back request to send in parts.")
	C.XChangeProperty(display, r.win, r.property, atomIncr,
		/* format: 32 bits */ 32 /* mode */, C.PropModeReplace, nil, 0)
	// Need to follow requestor property changes.
	C.XSelectInput(display, r.win, C.PropertyChangeMask)
	r.state = Incremental
	return false
}

func (r *requestHandler) SendContentPart() bool {
	missing := len(r.content) - r.position
	amount := missing
	if amount > targetPartSize {
		amount = targetPartSize
	}
	glog.V(2).Infof("SendContentPart(): %d bytes missing, sending %d.", missing, amount)
	if amount > 0 {
		C.XChangeProperty(display, r.win, r.property, atomPNGTarget,
			/* byte */ 8, C.PropModeReplace,
			(*C.uchar)(unsafe.Pointer(&r.content[r.position])), C.int(amount))
	} else {
		// Signals end of transfer.
		C.XChangeProperty(display, r.win, r.property, atomPNGTarget,
			/* byte */ 8, C.PropModeReplace, nil, 0)
	}
	C.XFlush(display)
	r.position += amount
	if amount == 0 {
		glog.V(2).Info("- transfer finished")
		r.state = Initial
	}
	return amount == 0
}

type requestState int

const (
	Initial requestState = iota
	Incremental
)

func getWindowName(win C.Window) string {
	if win == C.None {
		return "no window!"
	}

	var cName *C.char
	for win != C.None {
		// Try to get name of window.
		C.XFetchName(display, win, &cName)
		if cName != nil {
			name := C.GoString(cName)
			C.free(unsafe.Pointer(cName))
			return name
		}

		// If it doesn't, try to get parent tree recursively.
		var root, parent C.Window
		var children *C.Window
		var numChildren C.uint
		if C.XQueryTree(display, win, &root, &parent, &children, &numChildren) == C.BadWindow {
			break
		}
		win = parent
	}
	return "window with no name"
}

func getAtomFromName(name string) C.Atom {
	c := C.CString(name)
	defer C.free(unsafe.Pointer(c))
	return C.XInternAtom(display, c, C.False)
}

func getNameFromAtom(atom C.Atom) string {
	cStr := C.XGetAtomName(display, atom)
	defer C.free(unsafe.Pointer(cStr))
	return C.GoString(cStr)
}

// XEventType are the values in an C.XEvent, defined in X11.h.
type XEventType C.int

// List of event types is generated from /usr/include/X11/X.h.
const (
	Reserved0EventType XEventType = iota
	Reserved1EventType
	KeyPressEventType
	KeyReleaseEventType
	ButtonPressEventType
	ButtonReleaseEventType
	MotionNotifyEventType
	EnterNotifyEventType
	LeaveNotifyEventType
	FocusInEventType
	FocusOutEventType
	KeymapNotifyEventType
	ExposeEventType
	GraphicsExposeEventType
	NoExposeEventType
	VisibilityNotifyEventType
	CreateNotifyEventType
	DestroyNotifyEventType
	UnmapNotifyEventType
	MapNotifyEventType
	MapRequestEventType
	ReparentNotifyEventType
	ConfigureNotifyEventType
	ConfigureRequestEventType
	GravityNotifyEventType
	ResizeRequestEventType
	CirculateNotifyEventType
	CirculateRequestEventType
	PropertyNotifyEventType
	SelectionClearEventType
	SelectionRequestEventType
	SelectionNotifyEventType
	ColormapNotifyEventType
	ClientMessageEventType
	MappingNotifyEventType
	GenericEventEventType
	LASTEventEventType
)

// eventTypesNames is generated from /usr/include/X11/X.h.
var eventTypesNames = []string{
	"RESERVED_0",
	"RESERVED_1",
	"KeyPress",
	"KeyRelease",
	"ButtonPress",
	"ButtonRelease",
	"MotionNotify",
	"EnterNotify",
	"LeaveNotify",
	"FocusIn",
	"FocusOut",
	"KeymapNotify",
	"Expose",
	"GraphicsExpose",
	"NoExpose",
	"VisibilityNotify",
	"CreateNotify",
	"DestroyNotify",
	"UnmapNotify",
	"MapNotify",
	"MapRequest",
	"ReparentNotify",
	"ConfigureNotify",
	"ConfigureRequest",
	"GravityNotify",
	"ResizeRequest",
	"CirculateNotify",
	"CirculateRequest",
	"PropertyNotify",
	"SelectionClear",
	"SelectionRequest",
	"SelectionNotify",
	"ColormapNotify",
	"ClientMessage",
	"MappingNotify",
	"GenericEvent",
	"LASTEvent",
}
