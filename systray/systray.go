package systray

import (
	glst "github.com/getlantern/systray"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/resources"
	"os/exec"
	"strings"
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

func onScreenshot() {
	args := make([]string, 0, len(PreParseArgs))
	for _, arg := range PreParseArgs {
		if strings.Contains(arg, "systray") {
			// Remove the -systray flag.
			continue
		}
		args = append(args, arg)
	}

	err := exec.Command(args[0], args[1:]...).Start()
	if err != nil {
		glog.Errorf("Command attempted to execute: %v", args)
		glog.Errorf("Failed to start GoShot to screenshot: %s", err)
	}
}
