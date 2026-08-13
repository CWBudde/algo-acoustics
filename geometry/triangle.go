package geometry

import "math"

// Triangle is a flat polygon defined by three vertices in counter-clockwise
// order when viewed from the front (outward normal side).
type Triangle struct {
	V0, V1, V2 Vec3
}

// Normal returns the unit outward normal of the triangle.
func (t Triangle) Normal() Vec3 {
	return t.V1.Sub(t.V0).Cross(t.V2.Sub(t.V0)).Normalize()
}

// Centroid returns the geometric centre of the triangle.
func (t Triangle) Centroid() Vec3 {
	return t.V0.Add(t.V1).Add(t.V2).Scale(1.0 / 3)
}

// Area returns the surface area of the triangle.
func (t Triangle) Area() float64 {
	return t.V1.Sub(t.V0).Cross(t.V2.Sub(t.V0)).Norm() * 0.5
}

// pointInTriangle reports whether p lies on triangle tri, using barycentric
// coordinates with an absolute tolerance eps. The point must also be coplanar
// with the triangle to within eps.
func pointInTriangle(tri Triangle, p Vec3, eps float64) bool {
	e1 := tri.V1.Sub(tri.V0)
	e2 := tri.V2.Sub(tri.V0)

	n := e1.Cross(e2)

	norm := n.Norm()
	if norm == 0 {
		return false // degenerate triangle
	}

	vp := p.Sub(tri.V0)

	// Reject points off the triangle's plane.
	if math.Abs(vp.Dot(n)/norm) > eps {
		return false
	}

	d00 := e1.Dot(e1)
	d01 := e1.Dot(e2)
	d11 := e2.Dot(e2)
	d20 := vp.Dot(e1)
	d21 := vp.Dot(e2)

	denom := d00*d11 - d01*d01
	if denom == 0 {
		return false
	}

	u := (d11*d20 - d01*d21) / denom
	v := (d00*d21 - d01*d20) / denom

	return u >= -eps && v >= -eps && u+v <= 1+eps
}

// RayTriangle tests for intersection between ray r and triangle t using the
// Möller–Trumbore algorithm. Returns the ray parameter t and whether a hit
// occurred for t > 0. The test is double-sided, so front- and back-face hits
// are both accepted.
func RayTriangle(r Ray, tri Triangle) (t float64, hit bool) {
	const eps = 1e-9

	e1 := tri.V1.Sub(tri.V0)
	e2 := tri.V2.Sub(tri.V0)
	h := r.Direction.Cross(e2)
	det := e1.Dot(h)

	if math.Abs(det) < eps {
		return 0, false // ray is parallel to triangle
	}

	invDet := 1 / det
	s := r.Origin.Sub(tri.V0)
	u := s.Dot(h) * invDet

	if u < 0 || u > 1 {
		return 0, false
	}

	q := s.Cross(e1)
	v := r.Direction.Dot(q) * invDet

	if v < 0 || u+v > 1 {
		return 0, false
	}

	t = e2.Dot(q) * invDet
	if t < eps {
		return 0, false
	}

	return t, true
}
