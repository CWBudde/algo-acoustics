package hrtf

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestNearestNeighborDatasetImplementsDataset(t *testing.T) {
	t.Parallel()

	var _ Dataset = NearestNeighborDataset{}
}

func TestNearestNeighborDatasetSampleRate(t *testing.T) {
	t.Parallel()

	dataset := NearestNeighborDataset{SampleRateHz: 48000}
	if got := dataset.SampleRate(); got != 48000 {
		t.Fatalf("SampleRate() = %d, want 48000", got)
	}
}

func TestInterpolatingDatasetMarshalsEstablishedSampleRateField(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(InterpolatingDataset{SampleRateHz: 48000})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var fields map[string]any

	err = json.Unmarshal(data, &fields)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got := fields["sampleRate"]; got != float64(48000) {
		t.Fatalf("sampleRate = %v, want 48000", got)
	}

	if _, ok := fields["sample_rate"]; ok {
		t.Fatalf("marshaled dataset contains legacy sample_rate field: %s", data)
	}
}

func TestNearestNeighborDatasetLookupStub(t *testing.T) {
	t.Parallel()

	dataset := NearestNeighborDataset{SampleRateHz: 48000}

	left, right, delaySeconds, err := dataset.Lookup(geometry.Vec3{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Lookup() returned error: %v", err)
	}

	if len(left) != 1 || len(right) != 1 || left[0] != 1 || right[0] != 1 || delaySeconds != 0 {
		t.Fatalf("Lookup() = (%v, %v, %v), want identity impulse", left, right, delaySeconds)
	}
}

func TestNearestNeighborFrontDirection(t *testing.T) {
	t.Parallel()

	grid := &MeasurementGrid{
		Directions: []geometry.Vec3{{X: 1, Y: 0, Z: 0}, {X: -1, Y: 0, Z: 0}, {X: 0, Y: 1, Z: 0}},
		LeftHRIRs:  [][]float64{{1}, {2}, {3}},
		RightHRIRs: [][]float64{{4}, {5}, {6}},
		Delays:     []float64{0.01, 0.02, 0.03},
	}

	if got := NearestNeighbor(grid, geometry.Vec3{X: 1, Y: 0, Z: 0}); got != 0 {
		t.Fatalf("NearestNeighbor() = %d, want 0", got)
	}

	left, right, delay := LookupNearest(grid, geometry.Vec3{X: 1, Y: 0, Z: 0})
	if len(left) != 1 || len(right) != 1 || left[0] != 1 || right[0] != 4 || math.Abs(delay-0.01) > 1e-12 {
		t.Fatalf("LookupNearest() = (%v, %v, %v), want first measurement", left, right, delay)
	}
}

func TestLookupNearestReturnsPositiveDelayForLeftDirection(t *testing.T) {
	t.Parallel()

	grid := &MeasurementGrid{
		Directions: []geometry.Vec3{{X: 1, Y: 0, Z: 0}, {X: -1, Y: 0, Z: 0}},
		LeftHRIRs:  [][]float64{{1}, {1}},
		RightHRIRs: [][]float64{{1}, {1}},
		Delays:     []float64{0, 0.015},
	}

	left, right, delay := LookupNearest(grid, geometry.Vec3{X: -1, Y: 0, Z: 0})
	if len(left) != 1 || len(right) != 1 {
		t.Fatalf("LookupNearest() = (%v, %v, %v), want unit HRIRs", left, right, delay)
	}

	if delay <= 0 {
		t.Fatalf("LookupNearest() delay = %v, want positive delay for sound from the left", delay)
	}
}
