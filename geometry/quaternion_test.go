package geometry_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const quatEps = 1e-10

func vec3Near(a, b geometry.Vec3, eps float64) bool {
	return math.Abs(a.X-b.X) < eps &&
		math.Abs(a.Y-b.Y) < eps &&
		math.Abs(a.Z-b.Z) < eps
}

func TestQuatIdentityRotate(t *testing.T) {
	v := geometry.Vec3{1, 2, 3}
	got := geometry.QuatIdentity().Rotate(v)

	if !vec3Near(got, v, quatEps) {
		t.Errorf("identity rotate: got %v, want %v", got, v)
	}
}

func TestQuatFromZeroAxisReturnsIdentity(t *testing.T) {
	v := geometry.Vec3{X: 2, Y: -3, Z: 4}
	q := geometry.QuatFromAxisAngle(geometry.Vec3Zero, math.Pi/2)
	got := q.Rotate(v)

	if q != geometry.QuatIdentity() {
		t.Fatalf("QuatFromAxisAngle(zero, angle) = %#v, want identity", q)
	}

	if !vec3Near(got, v, quatEps) {
		t.Fatalf("zero-axis rotation = %v, want %v", got, v)
	}

	if math.Abs(got.Norm()-v.Norm()) > quatEps {
		t.Fatalf("zero-axis rotation changed length from %v to %v", v.Norm(), got.Norm())
	}
}

func TestQuatRotateXAxis90(t *testing.T) {
	// 90° around Z: (1,0,0) → (0,1,0)
	q := geometry.QuatFromAxisAngle(geometry.Vec3{0, 0, 1}, math.Pi/2)
	got := q.Rotate(geometry.Vec3{1, 0, 0})
	want := geometry.Vec3{0, 1, 0}

	if !vec3Near(got, want, quatEps) {
		t.Errorf("90° around Z: got %v, want %v", got, want)
	}
}

func TestQuatRotate180(t *testing.T) {
	// 180° around Z: (1,0,0) → (−1,0,0)
	q := geometry.QuatFromAxisAngle(geometry.Vec3{0, 0, 1}, math.Pi)
	got := q.Rotate(geometry.Vec3{1, 0, 0})
	want := geometry.Vec3{-1, 0, 0}

	if !vec3Near(got, want, quatEps) {
		t.Errorf("180° around Z: got %v, want %v", got, want)
	}
}

func TestQuatRotatePreservesLength(t *testing.T) {
	q := geometry.QuatFromAxisAngle(geometry.Vec3{1, 1, 1}, math.Pi/3)
	v := geometry.Vec3{2, 3, 4}
	got := q.Rotate(v)

	if math.Abs(got.Norm()-v.Norm()) > quatEps {
		t.Errorf("rotation changed vector length: %v → %v", v.Norm(), got.Norm())
	}
}

func TestQuatMulIdentity(t *testing.T) {
	q := geometry.QuatFromAxisAngle(geometry.Vec3{0, 1, 0}, math.Pi/4)
	id := geometry.QuatIdentity()

	qi := q.Mul(id)
	iq := id.Mul(q)

	v := geometry.Vec3{1, 2, 3}
	if !vec3Near(qi.Rotate(v), q.Rotate(v), quatEps) {
		t.Error("q * identity ≠ q")
	}

	if !vec3Near(iq.Rotate(v), q.Rotate(v), quatEps) {
		t.Error("identity * q ≠ q")
	}
}

func TestQuatRoundTrip(t *testing.T) {
	// Rotating by θ then by −θ should return the original vector.
	axis := geometry.Vec3{1, 1, 0}.Normalize()
	q := geometry.QuatFromAxisAngle(axis, math.Pi/3)
	qInv := geometry.QuatFromAxisAngle(axis, -math.Pi/3)

	v := geometry.Vec3{3, 1, 4}
	got := qInv.Rotate(q.Rotate(v))

	if !vec3Near(got, v, quatEps) {
		t.Errorf("round-trip: got %v, want %v", got, v)
	}
}
