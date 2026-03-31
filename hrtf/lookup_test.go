package hrtf

import (
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

func TestNearestNeighborDatasetLookupStub(t *testing.T) {
	t.Parallel()

	dataset := NearestNeighborDataset{SampleRateHz: 48000}
	left, right, delaySeconds, err := dataset.Lookup(geometry.Vec3{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Lookup() returned error: %v", err)
	}
	if left != nil || right != nil || delaySeconds != 0 {
		t.Fatalf("Lookup() = (%v, %v, %v), want (nil, nil, 0)", left, right, delaySeconds)
	}
}
