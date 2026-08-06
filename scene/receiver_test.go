package scene

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
)

func TestReceiverWorldToHeadDirIdentityOrientation(t *testing.T) {
	t.Parallel()

	receiver := Receiver{Orientation: geometry.QuatIdentity()}
	dir := receiver.WorldToHeadDir(geometry.Vec3{X: 1, Y: 0, Z: 0})

	if dir != (geometry.Vec3{X: 1, Y: 0, Z: 0}) {
		t.Fatalf("WorldToHeadDir() = %#v, want +X", dir)
	}
}

func TestReceiverNearestNeighborHRTFRoundTripPreservesGrid(t *testing.T) {
	t.Parallel()

	grid := &hrtf.MeasurementGrid{
		Directions: []geometry.Vec3{{X: 1}, {Y: 1}},
		LeftHRIRs:  [][]float64{{1, 0.25}, {0.5, 0.125}},
		RightHRIRs: [][]float64{{0.75, 0.125}, {1, 0.25}},
		Delays:     []float64{0, 0.0002},
		Triangles:  [][3]int{{0, 1, 0}},
	}
	receiver := Receiver{
		Type: ReceiverBinaural,
		HRTF: hrtf.NearestNeighborDataset{SampleRateHz: 48000, Grid: grid},
	}

	data, err := json.Marshal(receiver)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Receiver

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	dataset, ok := decoded.HRTF.(hrtf.NearestNeighborDataset)
	if !ok {
		t.Fatalf("decoded HRTF type = %T, want hrtf.NearestNeighborDataset", decoded.HRTF)
	}

	if !reflect.DeepEqual(dataset.Grid, grid) {
		t.Fatalf("decoded grid = %#v, want %#v", dataset.Grid, grid)
	}
}

func TestReceiverWorldToHeadDirRotatedOrientation(t *testing.T) {
	t.Parallel()

	receiver := Receiver{Orientation: geometry.QuatFromAxisAngle(geometry.Vec3{X: 0, Y: 0, Z: 1}, math.Pi/2)}
	dir := receiver.WorldToHeadDir(geometry.Vec3{X: 1, Y: 0, Z: 0})

	if math.Abs(dir.X) > 1e-9 || math.Abs(dir.Z) > 1e-9 {
		t.Fatalf("WorldToHeadDir() = %#v, want direction in XY plane", dir)
	}

	if math.Abs(dir.Y+1) > 1e-9 {
		t.Fatalf("WorldToHeadDir() = %#v, want -Y", dir)
	}
}

func TestReceiverWorldToHeadDirQuarterTurnSwapsLaterals(t *testing.T) {
	t.Parallel()

	receiver := Receiver{Orientation: geometry.QuatFromAxisAngle(geometry.Vec3{X: 0, Y: 0, Z: 1}, math.Pi/2)}
	left := receiver.WorldToHeadDir(geometry.Vec3{X: 0, Y: 1, Z: 0})
	right := receiver.WorldToHeadDir(geometry.Vec3{X: 0, Y: -1, Z: 0})

	if math.Abs(left.X-1) > 1e-9 || math.Abs(left.Y) > 1e-9 || math.Abs(left.Z) > 1e-9 {
		t.Fatalf("left direction = %#v, want +X", left)
	}

	if math.Abs(right.X+1) > 1e-9 || math.Abs(right.Y) > 1e-9 || math.Abs(right.Z) > 1e-9 {
		t.Fatalf("right direction = %#v, want -X", right)
	}
}
