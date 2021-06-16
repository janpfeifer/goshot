// +build !linux,!windows

package systray

func platformDependentInit() {
	glog.Fatal("Sorry, SysTray is not implemented for your OS version.")
}

func onScreenshot() {
	glog.Fatal("Sorry, SysTray is not implemented for your OS version.")
}
