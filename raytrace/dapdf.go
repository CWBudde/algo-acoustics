package raytrace

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const (
	// DAPDFV0 is the continuous join between the parabolic central lobe and
	// the rational outer branches of the deflection-angle density.
	DAPDFV0 = 0.5411961001461969

	dapdfOuterOffset = math.Sqrt2 - 1
)

// DAPDF evaluates the unscaled piecewise deflection density at v. v0 and d0
// are explicit to retain the formulation used by the RAVEN reference. The
// standard density uses DAPDFV0 and its normalizing constant from
// dapdfNormalization.
func DAPDF(v, v0, d0 float64) float64 {
	if v0 <= 0 || d0 < 0 || math.IsNaN(v) || math.IsNaN(v0) || math.IsNaN(d0) {
		return 0
	}

	if math.Abs(v) <= v0 {
		return d0 * math.Max(0, 1-v*v)
	}

	return d0 * 0.5 / (dapdfOuterOffset + v*v)
}

// DAPDFIntegral integrates the normalized angular density between epsilonMin
// and epsilonMax. b is the dimensionless apparent slit width; v=2*b*epsilon.
// Reversed limits produce a signed result.
func DAPDFIntegral(epsilonMin, epsilonMax, b float64) float64 {
	if !(b > 0) || math.IsNaN(epsilonMin) || math.IsNaN(epsilonMax) || math.IsInf(b, 0) {
		return 0
	}

	vMin := 2 * b * epsilonMin
	vMax := 2 * b * epsilonMax

	return (dapdfPrimitive(vMax) - dapdfPrimitive(vMin)) / dapdfTotalIntegral()
}

func dapdfPrimitive(v float64) float64 {
	if math.IsNaN(v) {
		return math.NaN()
	}

	if math.IsInf(v, 1) {
		return dapdfTotalIntegral() / 2
	}

	if math.IsInf(v, -1) {
		return -dapdfTotalIntegral() / 2
	}

	sign := 1.0

	x := v
	if x < 0 {
		sign = -1
		x = -x
	}

	if x <= DAPDFV0 {
		return sign * (x - x*x*x/3)
	}

	root := math.Sqrt(dapdfOuterOffset)
	center := DAPDFV0 - DAPDFV0*DAPDFV0*DAPDFV0/3
	tail := 0.5 / root * (math.Atan(x/root) - math.Atan(DAPDFV0/root))

	return sign * (center + tail)
}

func dapdfTotalIntegral() float64 {
	root := math.Sqrt(dapdfOuterOffset)
	center := DAPDFV0 - DAPDFV0*DAPDFV0*DAPDFV0/3
	tail := 0.5 / root * (math.Pi/2 - math.Atan(DAPDFV0/root))

	return 2 * (center + tail)
}

func dapdfNormalization() float64 { return 1 / dapdfTotalIntegral() }

// DeflectionCylinder is an open finite cylinder centered on a diffraction
// edge. Radius is frequency dependent and equals seven wavelengths.
type DeflectionCylinder struct {
	ID     int
	Edge   geometry.DiffractionEdge
	Radius float64
}

// NewDeflectionCylinder constructs the detector for wavelength metres.
func NewDeflectionCylinder(id int, edge geometry.DiffractionEdge, wavelength float64) DeflectionCylinder {
	radius := 0.0
	if wavelength > 0 && !math.IsInf(wavelength, 0) && !math.IsNaN(wavelength) {
		radius = 7 * wavelength
	}

	return DeflectionCylinder{ID: id, Edge: edge, Radius: radius}
}
