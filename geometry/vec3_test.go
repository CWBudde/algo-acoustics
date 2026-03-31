package geometry_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestVec3Add(t *testing.T) {
	got := geometry.Vec3{1, 2, 3}.Add(geometry.Vec3{4, 5, 6})
	want := geometry.Vec3{5, 7, 9}

	if got != want {
		t.Errorf("Add = %v, want %v", got, want)
	}
}

func TestVec3Sub(t *testing.T) {
	got := geometry.Vec3{5, 7, 9}.Sub(geometry.Vec3{4, 5, 6})
	want := geometry.Vec3{1, 2, 3}

	if got != want {
		t.Errorf("Sub = %v, want %v", got, want)
	}
}

func TestVec3Scale(t *testing.T) {
	got := geometry.Vec3{1, 2, 3}.Scale(2)
	want := geometry.Vec3{2, 4, 6}

	if got != want {
		t.Errorf("Scale = %v, want %v", got, want)
	}
}

func TestVec3Dot(t *testing.T) {
	got := geometry.Vec3{1, 0, 0}.Dot(geometry.Vec3{0, 1, 0})
	if got != 0 {
		t.Errorf("orthogonal dot = %v, want 0", got)
	}

	got = geometry.Vec3{1, 2, 3}.Dot(geometry.Vec3{1, 2, 3})
	if math.Abs(got-14) > 1e-12 {
		t.Errorf("self dot = %v, want 14", got)
	}
}

func TestVec3Cross(t *testing.T) {
	got := geometry.Vec3{1, 0, 0}.Cross(geometry.Vec3{0, 1, 0})
	want := geometry.Vec3{0, 0, 1}

	if got != want {
		t.Errorf("X×Y = %v, want %v", got, want)
	}
}

func TestVec3CrossAntiCommutative(t *testing.T) {
	a := geometry.Vec3{1, 2, 3}
	b := geometry.Vec3{4, 5, 6}

	ab := a.Cross(b)
	ba := b.Cross(a)

	if ab != ba.Neg() {
		t.Errorf("a×b ≠ −(b×a): %v vs %v", ab, ba.Neg())
	}
}

func TestVec3Norm(t *testing.T) {
	got := geometry.Vec3{3, 4, 0}.Norm()
	if math.Abs(got-5) > 1e-12 {
		t.Errorf("Norm = %v, want 5", got)
	}
}

func TestVec3Normalize(t *testing.T) {
	v := geometry.Vec3{3, 4, 0}.Normalize()
	if math.Abs(v.Norm()-1) > 1e-12 {
		t.Errorf("Normalize().Norm() = %v, want 1", v.Norm())
	}
}

func TestVec3NormalizeZero(t *testing.T) {
	got := geometry.Vec3Zero.Normalize()
	if got != geometry.Vec3Zero {
		t.Errorf("Normalize(zero) = %v, want zero", got)
	}
}

func TestVec3Distance(t *testing.T) {
	got := geometry.Vec3{0, 0, 0}.Distance(geometry.Vec3{3, 4, 0})
	if math.Abs(got-5) > 1e-12 {
		t.Errorf("Distance = %v, want 5", got)
	}
}

func TestVec3DistanceSymmetric(t *testing.T) {
	a := geometry.Vec3{1, 2, 3}
	b := geometry.Vec3{4, 5, 6}

	if math.Abs(a.Distance(b)-b.Distance(a)) > 1e-12 {
		t.Error("Distance is not symmetric")
	}
}

func TestVec3Neg(t *testing.T) {
	v := geometry.Vec3{1, -2, 3}
	got := v.Neg()
	want := geometry.Vec3{-1, 2, -3}

	if got != want {
		t.Errorf("Neg = %v, want %v", got, want)
	}
}
