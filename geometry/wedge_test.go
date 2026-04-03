package geometry

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestWedgeDiffractionHelpers(t *testing.T) {
	const s = 3.0
	const sPrime = 4.0
	const betaZero = math.Pi / 3

	gotSpread := wedgeSpreadingFactor(s, sPrime)
	wantSpread := 1 / math.Sqrt(84)
	if math.Abs(gotSpread-wantSpread) > 1e-12 {
		t.Fatalf("wedgeSpreadingFactor(%g, %g) = %g, want %g", s, sPrime, gotSpread, wantSpread)
	}

	gotL := wedgeDistanceParameter(s, sPrime, betaZero)
	wantL := 9.0 / 7.0
	if math.Abs(gotL-wantL) > 1e-12 {
		t.Fatalf("wedgeDistanceParameter(%g, %g, %g) = %g, want %g", s, sPrime, betaZero, gotL, wantL)
	}
}

func TestWedgeDiffractionHalfPlaneReference(t *testing.T) {
	got := WedgeDiffraction(2.2, 0.6, 1.1, 2.0, 7.5, 0.8)
	want := complex(-0.38540812329337376, 0.18636548524754248)

	if cmplx.Abs(got-want) > 5e-3 {
		t.Fatalf("WedgeDiffraction(...) = %v, want %v", got, want)
	}
}

func TestWedgeDiffractionNinetyDegreeWedgeReference(t *testing.T) {
	got := WedgeDiffraction(1.3, 0.4, 1.0, 1.5, 5.2, 0.9)
	want := complex(-0.1658551761309078, 0.1384373007043032)

	if cmplx.Abs(got-want) > 5e-3 {
		t.Fatalf("WedgeDiffraction(...) = %v, want %v", got, want)
	}
}
