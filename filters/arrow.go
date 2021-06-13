package filters

import (
	"image"
	"image/color"
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type Arrow struct {
	// From, To implement the starting and final point of the arrow:
	// arrow is pointing to the "To" direction.
	From, To image.Point

	// Color of the Arrow to be drawn.
	Color color.Color

	// Thickness of the Arrow to be drawn.
	Thickness float64

	// Rectangle enclosing arrow.
	rect image.Rectangle

	rebaseMatrix mgl64.Mat3
}

// NewArrow creates a new Arrow (or ellipsis) filter. It draws
// an ellipsis whose dimensions fit the given rectangle.
// You must specify the color and the thickness of the Arrow to be drawn.
func NewArrow(from, to image.Point, color color.Color, thickness float64) *Arrow {
	c := &Arrow{Color: color, Thickness: thickness}
	c.SetPoints(from, to)
	return c
}

func (c *Arrow) SetPoints(from, to image.Point) {
	if to.X == from.X && to.Y == from.Y {
		to.X += 1 // So that arrow is always at least 1 in size.
	}
	c.From, c.To = from, to
	c.rect = image.Rectangle{Min: from, Max: to}.Canon()
	c.rect.Min.X -= int(c.Thickness + 0.99)
	c.rect.Min.Y -= int(c.Thickness + 0.99)
	c.rect.Max.X += int(c.Thickness + 0.99)
	c.rect.Max.Y += int(c.Thickness + 0.99)

	// Calculate matrix that will rotate and translate a point
	// relative to the segment from c.From to c.To, with origin in
	// c.From.
	delta := c.To.Sub(c.From)
	vertex := mgl64.Vec2{float64(delta.X), float64(delta.Y)}
	direction := vertex.Normalize()
	angle := math.Atan2(direction.Y(), direction.X())

	c.rebaseMatrix = mgl64.Translate2D(-vertex.X(), -vertex.Y())
	c.rebaseMatrix = c.rebaseMatrix.Mul3(mgl64.HomogRotate2D(-angle))
}

// at is the function given to the filterImage object.
func (c *Arrow) at(x, y int, under color.Color) color.Color {
	if x > c.rect.Max.X || x < c.rect.Min.X || y > c.rect.Max.Y || y < c.rect.Min.Y {
		return under
	}

	// Move to coordinates on the segment defined from c.From to c.To.
	point := mgl64.Vec3{float64(x), float64(y), 1.0} // Homogeneous coordinates.
	point = c.rebaseMatrix.Mul3x1(point)
	if math.Abs(point.Y()) > c.Thickness {
		return under
	}
	return c.Color
}

// Apply implements the ImageFilter interface.
func (c *Arrow) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
