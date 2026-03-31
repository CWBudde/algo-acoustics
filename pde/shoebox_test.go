package pde

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestShoeboxModesAreSorted(t *testing.T) {
	t.Parallel()

	modes := ShoeboxModes(&scene.Shoebox{Width: 6, Depth: 4.5, Height: 2.8}, 2)
	if len(modes) == 0 {
		t.Fatal("ShoeboxModes returned no modes")
	}
	for i := 1; i < len(modes); i++ {
		if modes[i].Freq < modes[i-1].Freq {
			t.Fatalf("modes not sorted at %d: %v < %v", i, modes[i].Freq, modes[i-1].Freq)
		}
	}
}

func TestFirstAxialModeMatchesAnalyticalFrequency(t *testing.T) {
	t.Parallel()

	room := &scene.Shoebox{Width: 6, Depth: 4.5, Height: 2.8}
	modes := ShoeboxModes(room, 1)
	if len(modes) == 0 {
		t.Fatal("no modes returned")
	}
	want := 343.0 / (2 * room.Width)
	got := modes[0].Freq
	if math.Abs(got-want)/want > 0.02 {
		t.Fatalf("first mode freq = %v, want around %v", got, want)
	}
}

func TestTransferFunctionToTimeDomainReturnsSignal(t *testing.T) {
	t.Parallel()

	tf := &TransferFunction{
		Freqs: []float64{0, 100, 200, 300},
		H: []complex128{
			1 + 0i,
			0.8 + 0.1i,
			0.3 + 0.2i,
			0.1 + 0i,
		},
	}
	out := tf.ToTimeDomain(1000, 64)
	if len(out) != 64 {
		t.Fatalf("len(out) = %d, want 64", len(out))
	}
	if math.Abs(out[0]) < 1e-9 {
		t.Fatal("time-domain output looks silent")
	}
}

func TestSweepShoeboxReturnsSamples(t *testing.T) {
	t.Parallel()

	tf, err := SweepShoebox(&scene.Shoebox{Width: 6, Depth: 4.5, Height: 2.8}, geometry.Vec3{X: 1, Y: 1, Z: 1}, geometry.Vec3{X: 3, Y: 2, Z: 1.2}, SweepConfig{FreqMin: 20, FreqMax: 100, NumPoints: 8, BoundaryCondition: "neumann"})
	if err != nil {
		t.Fatalf("SweepShoebox failed: %v", err)
	}
	if tf == nil || len(tf.Freqs) != 8 || len(tf.H) != 8 {
		t.Fatalf("unexpected transfer function shape: %#v", tf)
	}
}
