package filters

import (
	"image"
	"image/color"
)

type Circle struct {
	// Dim defines the rectangle that will encompass the
	// circle/ellipse.
	Dim image.Rectangle

	// Color of the circle to be drawn.
	Color color.Color

	// Thickness of the circle to be drawn.
	Thickness float64

	// Center is generated automatically.
	Center Vec2

	// Internal dimensions.
	innerRadius, outerRadius Vec2
}

// NewCircle creates a new circle (or ellipsis) filter. It draws
// an ellipsis whose dimensions fit the given rectangle.
// You must specify the color and the thickness of the circle to be drawn.
func NewCircle(dim image.Rectangle, color color.Color, thickness float64) *Circle {
	c := &Circle{Color: color, Thickness: thickness}
	c.SetDim(dim)
	return c
}

func (c *Circle) SetDim(dim image.Rectangle) {
	c.Dim = dim
	center := c.Dim.Min.Add(c.Dim.Max).Div(2)
	c.Center = Vec2{float64(center.X), float64(center.Y)}
	c.outerRadius = Vec2{
		float64(c.Dim.Max.X) - c.Center.X(),
		float64(c.Dim.Max.Y) - c.Center.Y(),
	}
	c.innerRadius = Vec2{
		c.outerRadius.X() - c.Thickness,
		c.outerRadius.Y() - c.Thickness,
	}
}

// at is the function given to the filterImage object.
func (c *Circle) at(x, y int, under color.Color) color.Color {
	if x > c.Dim.Max.X || x < c.Dim.Min.X || y > c.Dim.Max.Y || y < c.Dim.Min.Y {
		return under
	}

	oDx := (float64(x) - c.Center.X()) / c.outerRadius.X()
	oDy := (float64(y) - c.Center.Y()) / c.outerRadius.Y()
	oDist := oDx*oDx + oDy*oDy
	iDx := (float64(x) - c.Center.X()) / c.innerRadius.X()
	iDy := (float64(y) - c.Center.Y()) / c.innerRadius.Y()
	iDist := iDx*iDx + iDy*iDy

	if oDist > 1 || iDist < 1 {
		return under
	}

	return c.Color
}

// Apply implements the ImageFilter interface.
func (c *Circle) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
