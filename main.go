package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/screenshot"
	"github.com/janpfeifer/goshot/systray"
	"os"
)

var (
	flagSysTray = flag.Bool("systray", false,
		"Set this flag to take the app run in a system tray, and respond to a "+
			"global shortcut to take screenshots.")
	flagHotkey = flag.String("hotkey", "win+control+s",
		"Hotkey to register to trigger a screenshot. It accepts any combination "+
			"of 'shift', 'control', 'win', 'alt' and normal key, separated by '+'. Eg.: "+
			"'win+control+s`. Only used in -systray mode.")
)

func main() {
	systray.PreParseArgs = make([]string, len(os.Args))
	copy(systray.PreParseArgs, os.Args)
	flag.Parse()
	if *flagSysTray {
		glog.Infof("Running in system tray.")
		systray.Run()
	} else {
		screenshot.Run()
	}
}
