package geometry

import (
	"math"
	"math/cmplx"
	"testing"
)

// Reference values from McNamara et al. (1990), Table 4.1.
// F(x) is the UTD Fresnel transition function.
var fresnelReferenceTable = []struct {
	x    float64
	magF float64 // |F(x)|
	argF float64 // arg(F(x)) in degrees
}{
	{0.001, 0.0354, 45.0},
	{0.01, 0.1118, 44.7},
	{0.1, 0.3460, 39.6},
	{0.3, 0.5653, 30.0},
	{0.5, 0.6746, 23.4},
	{0.8, 0.7644, 17.1},
	{1.0, 0.8019, 14.0},
	{2.0, 0.8952, 7.5},
	{3.0, 0.9305, 5.1},
	{5.0, 0.9599, 3.1},
	{10.0, 0.9839, 1.5},
}

func TestFresnelTransitionReferenceValues(t *testing.T) {
	for _, tc := range fresnelReferenceTable {
		got := FresnelTransition(tc.x)
		gotMag := cmplx.Abs(got)
		gotArg := cmplx.Phase(got) * 180 / math.Pi

		// Magnitude tolerance: 1% relative or 0.005 absolute (whichever is larger).
		magTol := math.Max(tc.magF*0.01, 0.005)
		if math.Abs(gotMag-tc.magF) > magTol {
			t.Errorf("F(%g): |F| = %f, want %f (tol %f)", tc.x, gotMag, tc.magF, magTol)
		}

		// Phase tolerance: 1 degree or 3% relative (whichever is larger).
		argTol := math.Max(math.Abs(tc.argF)*0.03, 1.0)
		if math.Abs(gotArg-tc.argF) > argTol {
			t.Errorf("F(%g): arg(F) = %f°, want %f° (tol %f°)", tc.x, gotArg, tc.argF, argTol)
		}
	}
}

func TestFresnelTransitionLargeXApproachesOne(t *testing.T) {
	for _, x := range []float64{50, 100, 1000, 1e6} {
		got := FresnelTransition(x)
		if cmplx.Abs(got-1) > 0.01 {
			t.Errorf("F(%g) = %v, want ≈ 1+0i", x, got)
		}
	}
}

func TestFresnelTransitionZeroInput(t *testing.T) {
	got := FresnelTransition(0)
	if cmplx.Abs(got) > 1e-10 {
		t.Errorf("F(0) = %v, want 0", got)
	}
}

func TestFresnelTransitionNegativeInput(t *testing.T) {
	got := FresnelTransition(-1)
	if cmplx.Abs(got) > 1e-10 {
		t.Errorf("F(-1) = %v, want 0", got)
	}
}

func TestFresnelTransitionSmoothNearRegimeBoundaries(t *testing.T) {
	// Check continuity at the regime boundaries (x = 0.3 and x = 10).
	checkSmooth := func(boundary float64) {
		t.Helper()

		delta := boundary * 0.001
		below := FresnelTransition(boundary - delta)
		above := FresnelTransition(boundary + delta)
		jump := cmplx.Abs(above - below)

		if jump > 0.002 {
			t.Errorf("discontinuity at x=%g: F(%g)=%v, F(%g)=%v, jump=%g",
				boundary, boundary-delta, below, boundary+delta, above, jump)
		}
	}

	checkSmooth(0.3)
	checkSmooth(10.0)
}

func TestFresnelTransitionMonotonicMagnitude(t *testing.T) {
	// |F(x)| should be monotonically increasing for x > 0.
	prev := cmplx.Abs(FresnelTransition(0.001))

	for x := 0.01; x <= 100; x *= 1.5 {
		cur := cmplx.Abs(FresnelTransition(x))
		if cur < prev-1e-10 {
			t.Errorf("|F(%g)| = %f < |F(prev)| = %f — not monotonic", x, cur, prev)
		}

		prev = cur
	}
}
