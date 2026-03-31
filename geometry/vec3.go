// Package geometry provides 3-D math primitives — vectors, rays, planes,
// boxes, quaternions, and intersection routines — used throughout
// algo-acoustics.
package geometry

import "math"

// Vec3 is a three-dimensional vector or point.
// It is intentionally a value type for cache efficiency in hot loops.
type Vec3 struct {
	X, Y, Z float64
}

// Vec3 convenience constructors / constants.
var (
	Vec3Zero = Vec3{0, 0, 0}
	Vec3One  = Vec3{1, 1, 1}
)

// Add returns v + w.
func (v Vec3) Add(w Vec3) Vec3 { return Vec3{v.X + w.X, v.Y + w.Y, v.Z + w.Z} }

// Sub returns v − w.
func (v Vec3) Sub(w Vec3) Vec3 { return Vec3{v.X - w.X, v.Y - w.Y, v.Z - w.Z} }

// Scale returns v * s.
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }

// Dot returns the dot product v · w.
func (v Vec3) Dot(w Vec3) float64 { return v.X*w.X + v.Y*w.Y + v.Z*w.Z }

// Cross returns the cross product v × w.
func (v Vec3) Cross(w Vec3) Vec3 {
	return Vec3{
		v.Y*w.Z - v.Z*w.Y,
		v.Z*w.X - v.X*w.Z,
		v.X*w.Y - v.Y*w.X,
	}
}

// Norm returns the Euclidean length of v.
func (v Vec3) Norm() float64 { return math.Sqrt(v.Dot(v)) }

// Normalize returns a unit vector in the direction of v.
// Returns Vec3Zero for a zero-length vector.
func (v Vec3) Normalize() Vec3 {
	n := v.Norm()
	if n == 0 {
		return Vec3Zero
	}

	return v.Scale(1 / n)
}

// Distance returns the Euclidean distance between v and w.
func (v Vec3) Distance(w Vec3) float64 { return v.Sub(w).Norm() }

// Neg returns −v.
func (v Vec3) Neg() Vec3 { return Vec3{-v.X, -v.Y, -v.Z} }
