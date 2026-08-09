package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestDAPDFContinuousAtV0(t *testing.T) {
	d0 := dapdfNormalization()
	delta := 1e-7
	left := DAPDF(DAPDFV0-delta, DAPDFV0, d0)
	right := DAPDF(DAPDFV0+delta, DAPDFV0, d0)
	if math.Abs(left-right) > 1e-6 {
		t.Fatalf("DAPDF join mismatch: left=%g right=%g", left, right)
	}

	leftSlope := (DAPDF(DAPDFV0, DAPDFV0, d0) - DAPDF(DAPDFV0-delta, DAPDFV0, d0)) / delta
	rightSlope := (DAPDF(DAPDFV0+delta, DAPDFV0, d0) - DAPDF(DAPDFV0, DAPDFV0, d0)) / delta
	if math.Abs(leftSlope-rightSlope) > 1e-5 {
		t.Fatalf("DAPDF derivative mismatch: left=%g right=%g", leftSlope, rightSlope)
	}
}

func TestDAPDFIntegralNormalizedSymmetricAndSigned(t *testing.T) {
	for _, b := range []float64{0.01, 0.5, 5, 100} {
		got := DAPDFIntegral(math.Inf(-1), math.Inf(1), b)
		if math.Abs(got-1) > 1e-12 {
			t.Fatalf("DAPDFIntegral(-Inf,+Inf,%g) = %g, want 1", b, got)
		}

		forward := DAPDFIntegral(-0.4, 0.7, b)
		reverse := DAPDFIntegral(0.7, -0.4, b)
		if math.Abs(forward+reverse) > 1e-12 {
			t.Fatalf("signed integrals for b=%g = %g and %g", b, forward, reverse)
		}

		if math.Abs(DAPDFIntegral(-0.7, -0.4, b)-DAPDFIntegral(0.4, 0.7, b)) > 1e-12 {
			t.Fatalf("DAPDFIntegral is not symmetric for b=%g", b)
		}
	}
}

func TestDeflectionCylinderRadius(t *testing.T) {
	dc := NewDeflectionCylinder(3, geometry.DiffractionEdge{}, 0.5)
	if dc.ID != 3 || dc.Radius != 3.5 {
		t.Fatalf("NewDeflectionCylinder() = %#v", dc)
	}
}
