package filters

import (
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
)

type Text struct {
	// Text to render.
	Text string

	// Center (horizontal and vertical) where to draw the text.
	Center image.Point

	// Color of the Text to be drawn.
	Color color.Color

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
func NewText(text string, center image.Point, color color.Color, size float64) *Text {
	c := &Text{
		Text:   text,
		Center: center,
		Color:  color,
		Size:   size}
	c.SetText(text)
	return c
}

func (c *Text) SetText(text string) {
	c.Text = text
	c.renderedText = image.NewRGBA(image.Rect(0, 0, 300, 100))
	point := fixed.Point26_6{fixed.Int26_6(c.Size * 64), fixed.Int26_6(c.Size * 64)}
	d := &font.Drawer{
		Dst:  c.renderedText,
		Src:  image.NewUniform(c.Color),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(text)
	c.rect = image.Rectangle{Min: c.Center, Max: c.Center.Add(c.renderedText.Rect.Max)}
}

// at is the function given to the filterImage object.
func (c *Text) at(x, y int, under color.Color) color.Color {
	if x > c.rect.Max.X || x < c.rect.Min.X || y > c.rect.Max.Y || y < c.rect.Min.Y {
		return under
	}
	return c.renderedText.At(x-c.rect.Min.X, y-c.rect.Min.Y)
}

// Apply implements the ImageFilter interface.
func (c *Text) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
