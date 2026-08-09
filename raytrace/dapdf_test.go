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

func TestDAPDFPublishedShapePoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    float64
		want float64
	}{
		{name: "peak", v: 0, want: 1},
		{name: "negative join", v: -DAPDFV0, want: 1 / math.Sqrt2},
		{name: "positive join", v: DAPDFV0, want: 1 / math.Sqrt2},
		{name: "outer lobe", v: 1, want: 1 / (2 * math.Sqrt2)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := DAPDF(test.v, DAPDFV0, 1)
			if math.Abs(got-test.want) > 1e-12 {
				t.Fatalf("DAPDF(%g, v0, 1) = %g, want %g", test.v, got, test.want)
			}
		})
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

func TestDAPDFIntegralMatchesQuadratureAcrossPiecewiseCases(t *testing.T) {
	b := 2.5
	join := DAPDFV0 / (2 * b)
	tests := []struct {
		name string
		low  float64
		high float64
	}{
		{name: "negative outer", low: -3 * join, high: -2 * join},
		{name: "negative join", low: -2 * join, high: -join / 2},
		{name: "center", low: -join / 2, high: join / 2},
		{name: "positive join", low: join / 2, high: 2 * join},
		{name: "positive outer", low: 2 * join, high: 3 * join},
		{name: "both joins", low: -2 * join, high: 2 * join},
	}

	const steps = 20000
	d0 := dapdfNormalization()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := (test.high - test.low) / steps
			var numerical float64

			for index := range steps {
				epsilon := test.low + (float64(index)+0.5)*step
				numerical += 2 * b * DAPDF(2*b*epsilon, DAPDFV0, d0) * step
			}

			got := DAPDFIntegral(test.low, test.high, b)
			if math.Abs(got-numerical) > 1e-8 {
				t.Fatalf("DAPDFIntegral(%g,%g,%g) = %.12g, quadrature %.12g", test.low, test.high, b, got, numerical)
			}
		})
	}
}

func TestDeflectionCylinderRadius(t *testing.T) {
	dc := NewDeflectionCylinder(3, geometry.DiffractionEdge{}, 0.5)
	if dc.ID != 3 || dc.Radius != 3.5 {
		t.Fatalf("NewDeflectionCylinder() = %#v", dc)
	}
}
