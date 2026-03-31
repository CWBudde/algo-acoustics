package hrtf

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestBarycentricWeightsAtMeasurementVertex(t *testing.T) {
	t.Parallel()

	tri := [3]geometry.Vec3{{X: 1}, {Y: 1}, {Z: 1}}
	weights := BarycentricWeights(geometry.Vec3{X: 1}, tri)

	if math.Abs(weights[0]-1) > 1e-12 || math.Abs(weights[1]) > 1e-12 || math.Abs(weights[2]) > 1e-12 {
		t.Fatalf("BarycentricWeights() = %v, want [1 0 0]", weights)
	}
}

func TestInterpolateHRIREqualsMeasurementAtExactDirection(t *testing.T) {
	t.Parallel()

	grid := &MeasurementGrid{
		Directions: []geometry.Vec3{{X: 1, Y: 0, Z: 0}, {X: 0, Y: 1, Z: 0}, {X: 0, Y: 0, Z: 1}},
		LeftHRIRs:  [][]float64{{1, 2}, {3, 4}, {5, 6}},
		RightHRIRs: [][]float64{{7, 8}, {9, 10}, {11, 12}},
		Delays:     []float64{0.01, 0.02, 0.03},
	}

	left, right, delay := InterpolateHRIR(grid, geometry.Vec3{X: 0, Y: 1, Z: 0})
	if len(left) != 2 || left[0] != 3 || left[1] != 4 {
		t.Fatalf("left HRIR = %v, want exact measurement", left)
	}
	if len(right) != 2 || right[0] != 9 || right[1] != 10 {
		t.Fatalf("right HRIR = %v, want exact measurement", right)
	}
	if math.Abs(delay-0.02) > 1e-12 {
		t.Fatalf("delay = %v, want 0.02", delay)
	}
}
