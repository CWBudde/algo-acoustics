package geometry

// Box is an axis-aligned bounding box defined by two corner points.
type Box struct {
	Min, Max Vec3
}

// NewBox constructs a Box, ensuring Min ≤ Max component-wise.
func NewBox(a, b Vec3) Box {
	return Box{
		Min: Vec3{
			X: min64(a.X, b.X),
			Y: min64(a.Y, b.Y),
			Z: min64(a.Z, b.Z),
		},
		Max: Vec3{
			X: max64(a.X, b.X),
			Y: max64(a.Y, b.Y),
			Z: max64(a.Z, b.Z),
		},
	}
}

// Contains reports whether point p is inside or on the surface of the box.
func (b Box) Contains(p Vec3) bool {
	return p.X >= b.Min.X && p.X <= b.Max.X &&
		p.Y >= b.Min.Y && p.Y <= b.Max.Y &&
		p.Z >= b.Min.Z && p.Z <= b.Max.Z
}

// Center returns the geometric centre of the box.
func (b Box) Center() Vec3 {
	return b.Min.Add(b.Max).Scale(0.5)
}

// Dimensions returns the side lengths along each axis.
func (b Box) Dimensions() Vec3 {
	return b.Max.Sub(b.Min)
}

// Volume returns the box volume.
func (b Box) Volume() float64 {
	d := b.Dimensions()

	return d.X * d.Y * d.Z
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}

	return b
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}

	return b
}
