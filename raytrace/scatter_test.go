package raytrace

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// chiSquaredTest performs a chi-squared goodness-of-fit test.
// Returns the chi-squared statistic and the DoF.
func chiSquaredTest(observed, expected []float64) (float64, int) {
	var chi2 float64
	for i := range observed {
		diff := observed[i] - expected[i]
		chi2 += (diff * diff) / expected[i]
	}
	return chi2, len(observed) - 1
}

// criticalChiSquared returns the approximate critical value for chi-squared test.
// For DoF > 0, uses Choi's approximation valid for p=0.05 significance level.
// See: "The median of the chi-squared distribution" by Choi (2008).
func criticalChiSquared(dof int) float64 {
	if dof <= 0 {
		return 0
	}
	// Approximation for p=0.05: chi2_crit ≈ dof * (1 - 2/(9*dof) + 1.96*sqrt(2/(9*dof)))^3
	x := 1.0 - 2.0/(9.0*float64(dof)) + 1.96*math.Sqrt(2.0/(9.0*float64(dof)))
	return float64(dof) * x * x * x
}


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
	for range 64 {
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
