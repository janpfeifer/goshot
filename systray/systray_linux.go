package systray

import (
	"github.com/golang/glog"
	"os"
	"strings"
	"syscall"
)

//+build linux

// No init necessary for linux.
func platformDependentInit() {}

func onScreenshot() {
	args := make([]string, 0, len(PreParseArgs))
	for _, arg := range PreParseArgs {
		if strings.Contains(arg, "systray") {
			// Remove the -systray flag.
			continue
		}
		args = append(args, arg)
	}

	procAttrs := &syscall.ProcAttr{
		"",
		os.Environ(),
		[]uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		nil,
	}

	pid, err := syscall.ForkExec(args[0], args[1:], procAttrs)
	glog.Infof("Screenshot executed: %q, %v -> pid=%d", args[0], args[1:], pid)
	if err != nil {
		glog.Errorf("Failed to execute screenshot %q: %s", args[0], err)
	}
}
