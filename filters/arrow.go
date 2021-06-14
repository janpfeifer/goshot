package filters

import (
	"github.com/golang/glog"
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
	vectorLength float64
}

const (
	arrowHeadLengthFactor = 10.0
	arrowHeadWidthFactor  = 6.0
)

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
	arrowHeadExtraPixels := int(arrowHeadWidthFactor*c.Thickness + 0.99)
	c.rect.Min.X -= arrowHeadExtraPixels
	c.rect.Min.Y -= arrowHeadExtraPixels
	c.rect.Max.X += arrowHeadExtraPixels
	c.rect.Max.Y += arrowHeadExtraPixels

	// Calculate matrix that will rotate and translate a point
	// relative to the segment from c.From to c.To, with origin in
	// c.From.
	delta := c.To.Sub(c.From)
	vector := mgl64.Vec2{float64(delta.X), float64(delta.Y)}
	c.vectorLength = vector.Len()
	direction := vector.Mul(1.0 / c.vectorLength)
	angle := math.Atan2(direction.Y(), direction.X())
	glog.V(2).Infof("SetPoints(from=%v, to=%v): delta=%v, length=%.0f, angle=%5.1f",
		from, to, delta, c.vectorLength, mgl64.RadToDeg(angle))

	c.rebaseMatrix = mgl64.HomogRotate2D(-angle)
	c.rebaseMatrix = c.rebaseMatrix.Mul3(
		mgl64.Translate2D(float64(-c.From.X), float64(-c.From.Y)))
}

var (
	Yellow = color.RGBA{R: 255, G: 255, A: 255}
	Green  = color.RGBA{R: 80, G: 255, A: 80}
)

// at is the function given to the filterImage object.
func (c *Arrow) at(x, y int, under color.Color) color.Color {
	if x > c.rect.Max.X || x < c.rect.Min.X || y > c.rect.Max.Y || y < c.rect.Min.Y {
		return under
	}

	// Move to coordinates on the segment defined from c.From to c.To.
	homogPoint := mgl64.Vec3{float64(x), float64(y), 1.0} // Homogeneous coordinates.
	if glog.V(3) {
		if math.Abs(homogPoint.Y()-float64(c.To.Y)) < 2 || math.Abs(homogPoint.X()-float64(c.To.X)) < 2 {
			return Yellow
		}
		if math.Abs(homogPoint.Y()-float64(c.From.Y)) < 2 || math.Abs(homogPoint.X()-float64(c.From.X)) < 2 {
			return Yellow
		}
	}
	homogPoint = c.rebaseMatrix.Mul3x1(homogPoint)
	if glog.V(3) {
		if math.Abs(homogPoint.Y()) < 3 {
			return Green
		}
		if math.Abs(homogPoint.X()) < 1 {
			return Green
		}
		if math.Abs(homogPoint.X()-c.vectorLength) < 1 {
			return Green
		}
	}

	if homogPoint.X() < 0 {
		return under
	}
	if homogPoint.X() < c.vectorLength-arrowHeadLengthFactor*c.Thickness {
		if math.Abs(homogPoint.Y()) < c.Thickness/2 {
			return c.Color
		}
	} else {
		if math.Abs(homogPoint.Y()) < (c.vectorLength-homogPoint.X())*arrowHeadWidthFactor/arrowHeadLengthFactor/2.0 {
			return c.Color
		}
	}
	return under
}

// Apply implements the ImageFilter interface.
func (c *Arrow) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
