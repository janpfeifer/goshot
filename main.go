package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/janpfeifer/goshot/screenshot"
)

var (
	flagSysTray = flag.Bool("systray", false,
		"Set this flag to take the app run in a system tray, and respond to a "+
			"global shortcut to take screenshots.")
)

func main() {
	flag.Parse()
	if *flagSysTray {
		glog.Infof("Running in system tray.")
	} else {
		screenshot.Run()
	}
}
