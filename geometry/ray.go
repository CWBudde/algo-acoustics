package geometry

// Ray is a half-line defined by an origin point and a unit direction vector.
type Ray struct {
	Origin    Vec3
	Direction Vec3 // must be a unit vector
}

// NewRay constructs a Ray, normalising the direction.
func NewRay(origin, direction Vec3) Ray {
	return Ray{Origin: origin, Direction: direction.Normalize()}
}

// At returns the point along the ray at parameter t: Origin + t·Direction.
func (r Ray) At(t float64) Vec3 {
	return r.Origin.Add(r.Direction.Scale(t))
}
