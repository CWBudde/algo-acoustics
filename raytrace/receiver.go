package raytrace

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// SphereReceiver models a spherical capture region around the receiver.
type SphereReceiver struct {
	Center geometry.Vec3
	Radius float64
}

// Intersects reports the first intersection of r with the sphere in [tMin, tMax].
func (s SphereReceiver) Intersects(r geometry.Ray, tMin, tMax float64) (float64, bool) {
	if s.Radius <= 0 {
		return 0, false
	}

	oc := r.Origin.Sub(s.Center)
	a := r.Direction.Dot(r.Direction)
	b := 2 * oc.Dot(r.Direction)
	c := oc.Dot(oc) - s.Radius*s.Radius
	discriminant := b*b - 4*a*c
	if discriminant < 0 {
		return 0, false
	}

	sqrtDiscriminant := math.Sqrt(discriminant)
	t0 := (-b - sqrtDiscriminant) / (2 * a)
	if t0 >= tMin && t0 <= tMax {
		return t0, true
	}
	t1 := (-b + sqrtDiscriminant) / (2 * a)
	if t1 >= tMin && t1 <= tMax {
		return t1, true
	}

	return 0, false
}

// AngularWeight returns the optional angular weighting for a ray direction.
func (s SphereReceiver) AngularWeight(dir geometry.Vec3) float64 {
	if dir.Norm() == 0 {
		return 0
	}

	return 1
}
