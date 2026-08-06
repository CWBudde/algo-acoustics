package hrtf

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestInterpolatingDatasetImplementsDataset(t *testing.T) {
	t.Parallel()

	var _ Dataset = InterpolatingDataset{}
}

func TestBarycentricWeightsAtMeasurementVertex(t *testing.T) {
	t.Parallel()

	tri := [3]geometry.Vec3{{X: 1}, {Y: 1}, {Z: 1}}
	weights := BarycentricWeights(geometry.Vec3{X: 1}, tri)

	if math.Abs(weights[0]-1) > 1e-12 || math.Abs(weights[1]) > 1e-12 || math.Abs(weights[2]) > 1e-12 {
		t.Fatalf("BarycentricWeights() = %v, want [1 0 0]", weights)
	}
}

func TestBarycentricWeightsInsideMeasurementTriangle(t *testing.T) {
	t.Parallel()

	tri := [3]geometry.Vec3{{X: 1}, {Y: 1}, {Z: 1}}
	want := [3]float64{0.2, 0.3, 0.5}
	direction := tri[0].Scale(want[0]).Add(tri[1].Scale(want[1])).Add(tri[2].Scale(want[2])).Normalize()
	weights := BarycentricWeights(direction, tri)

	for index := range weights {
		if math.Abs(weights[index]-want[index]) > 1e-12 {
			t.Fatalf("BarycentricWeights() = %v, want %v", weights, want)
		}
	}
}

func TestInterpolateHRIRInsideExplicitTriangle(t *testing.T) {
	t.Parallel()

	grid := &MeasurementGrid{
		Directions: []geometry.Vec3{{X: 1}, {Y: 1}, {Z: 1}},
		LeftHRIRs:  [][]float64{{1, 2}, {3, 4}, {5, 6}},
		RightHRIRs: [][]float64{{6, 5}, {4, 3}, {2, 1}},
		Delays:     []float64{0.01, 0.02, 0.04},
		Triangles:  [][3]int{{0, 1, 2}},
	}
	wantWeights := [3]float64{0.2, 0.3, 0.5}
	direction := grid.Directions[0].Scale(wantWeights[0]).
		Add(grid.Directions[1].Scale(wantWeights[1])).
		Add(grid.Directions[2].Scale(wantWeights[2])).Normalize()

	left, right, delay := InterpolateHRIR(grid, direction)
	assertFloatSlicesClose(t, left, []float64{3.6, 4.6})
	assertFloatSlicesClose(t, right, []float64{3.4, 2.4})

	wantDelay := 0.028
	if math.Abs(delay-wantDelay) > 1e-12 {
		t.Fatalf("delay = %v, want %v", delay, wantDelay)
	}
}

func TestInterpolatingDatasetRequiresExplicitTopology(t *testing.T) {
	t.Parallel()

	grid := &MeasurementGrid{
		Directions: []geometry.Vec3{{X: 1}, {Y: 1}, {Z: 1}},
		LeftHRIRs:  [][]float64{{1}, {3}, {5}},
		RightHRIRs: [][]float64{{2}, {4}, {6}},
	}
	direction := geometry.Vec3{X: 0.8, Y: 0.1, Z: 0.1}.Normalize()
	dataset := InterpolatingDataset{SampleRateHz: 48000, Grid: grid}

	left, right, _, err := dataset.Lookup(direction)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	assertFloatSlicesClose(t, left, []float64{1})
	assertFloatSlicesClose(t, right, []float64{2})
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

func assertFloatSlicesClose(t *testing.T, got, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("slice length = %d, want %d", len(got), len(want))
	}

	for index := range got {
		if math.Abs(got[index]-want[index]) > 1e-12 {
			t.Fatalf("slice[%d] = %v, want %v (full slice %v)", index, got[index], want[index], got)
		}
	}
}
