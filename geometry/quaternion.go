package geometry

import "math"

// Quaternion represents a unit quaternion for 3-D rotations.
// The convention is q = W + Xi + Yj + Zk.
type Quaternion struct {
	W, X, Y, Z float64
}

// QuatIdentity returns the identity quaternion (no rotation).
func QuatIdentity() Quaternion { return Quaternion{W: 1} }

// QuatFromAxisAngle returns a quaternion representing a rotation of angleRad
// radians around axis (which need not be normalised).
func QuatFromAxisAngle(axis Vec3, angleRad float64) Quaternion {
	a := axis.Normalize()
	s := math.Sin(angleRad / 2)

	return Quaternion{
		W: math.Cos(angleRad / 2),
		X: a.X * s,
		Y: a.Y * s,
		Z: a.Z * s,
	}
}

// Mul returns the Hamilton product q * r.
func (q Quaternion) Mul(r Quaternion) Quaternion {
	return Quaternion{
		W: q.W*r.W - q.X*r.X - q.Y*r.Y - q.Z*r.Z,
		X: q.W*r.X + q.X*r.W + q.Y*r.Z - q.Z*r.Y,
		Y: q.W*r.Y - q.X*r.Z + q.Y*r.W + q.Z*r.X,
		Z: q.W*r.Z + q.X*r.Y - q.Y*r.X + q.Z*r.W,
	}
}

// Conj returns the conjugate of q (equivalent to the inverse for unit quaternions).
func (q Quaternion) Conj() Quaternion { return Quaternion{W: q.W, X: -q.X, Y: -q.Y, Z: -q.Z} }

// Rotate applies the rotation represented by q to the vector v.
// Computes q * pure(v) * q⁻¹.
func (q Quaternion) Rotate(v Vec3) Vec3 {
	p := Quaternion{W: 0, X: v.X, Y: v.Y, Z: v.Z}
	r := q.Mul(p).Mul(q.Conj())

	return Vec3{X: r.X, Y: r.Y, Z: r.Z}
}
