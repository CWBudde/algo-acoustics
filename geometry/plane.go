package geometry

// Plane is an infinite flat surface defined by a unit normal and a signed
// distance d from the origin, so that n·x = d for all points x on the plane.
type Plane struct {
	Normal   Vec3
	Distance float64
}

// NewPlaneFromPointNormal constructs a Plane from any point on the plane and
// an outward-facing normal (which is normalised internally).
func NewPlaneFromPointNormal(point, normal Vec3) Plane {
	n := normal.Normalize()

	return Plane{Normal: n, Distance: n.Dot(point)}
}

// SideOf returns a positive value if p is on the side the normal points toward,
// negative if behind, zero if on the plane.
func (p Plane) SideOf(point Vec3) float64 {
	return p.Normal.Dot(point) - p.Distance
}

// Reflect reflects the direction vector v about the plane's normal.
// Equivalent to the specular reflection law: v' = v − 2(v·n)n.
func (p Plane) Reflect(v Vec3) Vec3 {
	return v.Sub(p.Normal.Scale(2 * v.Dot(p.Normal)))
}
