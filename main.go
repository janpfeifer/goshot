package main

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/kbinani/screenshot"
	//"image/png"
	//"os"
)

func main() {
	n := screenshot.NumActiveDisplays()
	if n != 1 {
		glog.Warningf("No support for multiple displays yet (should be relatively easy to add), screenshotting first display.")
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Bounds: %+v\n", bounds)
	fmt.Printf("Image Rect: %v\n", img.Rect)
	//
	//fileName := fmt.Sprintf("%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
	//file, _ := os.Create(fileName)
	//defer file.Close()
	//png.Encode(file, img)
	//
	//fmt.Printf("#%d : %v \"%s\"\n", i, bounds, fileName)
}
