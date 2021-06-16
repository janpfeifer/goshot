package systray

import (
	glst "github.com/getlantern/systray"
	"github.com/getlantern/systray/example/icon"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/resources"
)

const appIconResID = 7

// PreParseArgs are the program arguments (os.Args()) before
// running flags.Parse. Notably glog library changes os.Args to remove
// its flags, but we want those flags to, when ForkExec'ing for
// the snapshot.
var PreParseArgs []string

// Run runs program as a system tray. It uses PreParseArgs when
// ForkExec'ing itself to do the snapshot, so it must be set accordingly.
func Run() {
	if len(PreParseArgs) == 0 {
		glog.Fatal("systray.Run must be run after setting PreParseArgs.")
	}
	platformDependentInit()
	glst.Run(onReady, onExit)
}

func onReady() {
	glst.SetIcon(resources.GoShotIconIco.Content())
	glst.SetTitle("GoShot (SysTray)")
	glst.SetTooltip("Take screenshot, edit and share!")

	mScreenshot := glst.AddMenuItem("Screenshot", "Take screenshot, edit and share!")
	go handler(mScreenshot, onScreenshot)
	mQuit := glst.AddMenuItem("Quit", "Quit the whole app")
	go func() { <-mQuit.ClickedCh; glst.Quit() }()

	// Sets the icon of a menu item. Only available on Mac and Windows.
	mQuit.SetIcon(icon.Data)

}

func onExit() {
	glog.Infof("Exiting GoShot system tray app.")
}

func handler(item *glst.MenuItem, onClick func()) {
	for {
		_, ok := <-item.ClickedCh
		if !ok { // Channel closed, return
			return
		}
		onClick()
	}
}
