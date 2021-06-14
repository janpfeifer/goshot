package filters

import (
	"github.com/golang/freetype/truetype"
	"github.com/golang/glog"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"strings"
)

// DPI constant. Ideally it would be read from the various system.
const DPI = 96

type Text struct {
	// Text to render.
	Text string

	// Center (horizontal and vertical) where to draw the text.
	Center image.Point

	// Color of the Text to be drawn.
	Color, Background color.Color

	// Font size.
	Size float64

	// Rectangle enclosing text.
	rect image.Rectangle

	// Text rendered.
	renderedText *image.RGBA
}

// NewText creates a new Text (or ellipsis) filter. It draws
// an ellipsis whose dimensions fit the given rectangle.
// You must specify the color and the thickness of the Text to be drawn.
func NewText(text string, center image.Point, color, background color.Color, size float64) *Text {
	c := &Text{
		Text:   text,
		Center: center,
		Color:  color,
		Background: background,
		Size:   size}

	c.SetText(text)
	return c
}

func (t *Text) SetText(text string) {
	t.Text = text
	point := fixed.Point26_6{X: 0, Y: fixed.Int26_6(t.Size * 64)}
	goBoldFont, err := truetype.Parse(gobold.TTF)
	if err != nil {
		glog.Fatalf("Failed to generate font for golang.org/x/image/font/gofont/gobold TTF.")
	}
	d := &font.Drawer{
		Dst: t.renderedText,
		Src: image.NewUniform(t.Color),
		Face: truetype.NewFace(goBoldFont, &truetype.Options{
			Size:       t.Size,
			DPI:        DPI,
			Hinting:    font.HintingFull,
			SubPixelsX: 8,
			SubPixelsY: 8,
		}),
		Dot: point,
	}

	// Handle multi-line content.
	margins := int((t.Size * DPI / 100.0) / 2.0 + 0.99)
	lines := strings.Split(text, "\n")
	var boundingRect image.Rectangle
	for _, line := range lines {
		lineRect, _ := d.BoundString(line)
		boundingRect.Max.Y += lineRect.Max.Y.Ceil() + margins
		if lineRect.Max.X.Ceil() > boundingRect.Max.X {
			boundingRect.Max.X = lineRect.Max.X.Ceil()
		}
	}
	boundingRect.Max.X += 2 * margins
	boundingRect.Max.Y += margins

	// Prepare background.
	img := image.NewRGBA(boundingRect)
	t.renderedText = img
	to8 := func(v uint32) uint8 { return uint8(v >> 8) }
	for ii := 0; ii < len(img.Pix); ii += 4 {
		r, g, b, a := t.Background.RGBA()
		img.Pix[ii] = to8(r)
		img.Pix[ii+1] = to8(g)
		img.Pix[ii+2] = to8(b)
		img.Pix[ii+3] = to8(a)
	}
	d.Dst = t.renderedText

	// Draw lines.
	for ii, line := range lines {
		d.Dot = fixed.Point26_6{
			X: fixed.Int26_6(margins*64),
			Y: fixed.Int26_6(((float64(ii+1) * (t.Size + float64(margins))) * 64))}
		d.DrawString(line)
	}

	normalizeAlpha(t.renderedText)

	cx, cy := t.Center.Y, t.Center.Y
	dx, dy := t.renderedText.Rect.Dx(), t.renderedText.Rect.Dy()
	t.rect = image.Rect(cx-dx/2, cy-dy/2, cx+dx/2, cy+dy/2)
}

func normalizeAlpha(img *image.RGBA) {
	var maxAlpha uint8
	for ii := 0; ii < len(img.Pix); ii += 4 {
		alpha := img.Pix[ii+3]
		if alpha > maxAlpha {
			maxAlpha = alpha
		}
	}
	const M = 1<<8 - 1
	maxAlpha16 := uint16(maxAlpha)
	for ii := 0; ii < len(img.Pix); ii += 4 {
		img.Pix[ii+3] = uint8(uint16(img.Pix[ii+3]) * M / maxAlpha16)
	}
}

var alphas = make(map[uint32]bool)

// at is the function given to the filterImage object.
func (t *Text) at(x, y int, under color.Color) color.Color {
	if x > t.rect.Max.X || x < t.rect.Min.X || y > t.rect.Max.Y || y < t.rect.Min.Y {
		return under
	}

	c := t.renderedText.At(x-t.rect.Min.X, y-t.rect.Min.Y)
	fontR, fontG, fontB, a := c.RGBA()
	if a == 0 {
		return under
	}
	const M = 1<<16 - 1

	underR, underG, underB, underA := under.RGBA()
	blend := func(underChan uint32, fontChan uint32) uint8 {
		return uint8((fontChan*a + underChan*(M-a)) / M >> 8)
	}
	return color.RGBA{
		R: blend(underR, fontR),
		G: blend(underG, fontG),
		B: blend(underB, fontB),
		A: uint8(underA >> 8),
	}
}

// Apply implements the ImageFilter interface.
func (t *Text) Apply(image image.Image) image.Image {
	return &filterImage{image, t.at}
}
