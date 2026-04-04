package raytrace

import (
	"math"
	"testing"
)

func TestBarrierInsertionLossCurvesStayWithinOneDb(t *testing.T) {
	t.Parallel()

	for _, fresnelNumber := range []float64{1.2, 2, 4, 8, 16} {
		maekawa := maekawaInsertionLossDb(fresnelNumber)
		iso := isoBarrierInsertionLossDb(fresnelNumber)

		if math.IsNaN(maekawa) || math.IsInf(maekawa, 0) {
			t.Fatalf("Maekawa(%g) = %v, want finite", fresnelNumber, maekawa)
		}

		if math.IsNaN(iso) || math.IsInf(iso, 0) {
			t.Fatalf("ISO(%g) = %v, want finite", fresnelNumber, iso)
		}

		if diff := math.Abs(maekawa - iso); diff > 1.0 {
			t.Fatalf("Maekawa(%g) - ISO(%g) = %.3f dB, want <= 1 dB", fresnelNumber, fresnelNumber, diff)
		}
	}
}

func maekawaInsertionLossDb(fresnelNumber float64) float64 {
	if fresnelNumber <= 0 {
		return 0
	}

	x := math.Sqrt(2 * math.Pi * fresnelNumber)
	return 5 + 20*math.Log10(x/math.Tanh(x))
}

func isoBarrierInsertionLossDb(fresnelNumber float64) float64 {
	if fresnelNumber <= 0 {
		return 0
	}

	return 10 * math.Log10(3+20*fresnelNumber)
}
