package raytrace

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestSpecularReflect(t *testing.T) {
	t.Parallel()

	reflected := SpecularReflect(geometry.Vec3{X: 1}, geometry.Vec3{X: 1})
	if reflected != (geometry.Vec3{X: -1}) {
		t.Fatalf("SpecularReflect() = %#v, want -X", reflected)
	}
}

func TestDiffuseReflectStaysInHemisphere(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 64; i++ {
		reflected := DiffuseReflect(geometry.Vec3{Z: 1}, rng)
		if reflected.Z <= 0 {
			t.Fatalf("DiffuseReflect() produced vector in wrong hemisphere: %#v", reflected)
		}
		if diff := math.Abs(reflected.Norm() - 1); diff > 1e-12 {
			t.Fatalf("DiffuseReflect() norm = %g, want 1", reflected.Norm())
		}
	}
}

func TestAbsorbedFractionClampsRange(t *testing.T) {
	t.Parallel()

	got := AbsorbedFraction([]float64{-0.25, 0.5, 1.25})
	want := []float64{1, 0.5, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %g, want %g", i, got[i], want[i])
		}
	}
}
