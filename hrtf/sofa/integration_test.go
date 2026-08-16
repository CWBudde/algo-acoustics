package sofa

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	gosofa "github.com/cwbudde/go-sofa"
)

// itdFixture builds a file whose lateral measurements carry a real
// interaural time difference: for a source on the left, the left ear's
// impulse arrives itdSamples earlier than the right ear's.
func itdFixture(itdSamples int) fixture {
	const length = 64

	f := sphericalFixture()
	f.samples = length
	f.positions = []gosofa.Vector3{
		{X: 90, Y: 0, Z: 1.5},  // left
		{X: 270, Y: 0, Z: 1.5}, // right
		{X: 0, Y: 0, Z: 1.5},   // front
	}

	f.irs = make([][][]float64, len(f.positions))
	for m := range f.irs {
		left := make([]float64, length)
		right := make([]float64, length)

		switch m {
		case 0: // source on the left: left ear leads
			left[0] = 1
			right[itdSamples] = 1
		case 1: // source on the right: right ear leads
			left[itdSamples] = 1
			right[0] = 1
		default: // straight ahead: no difference
			left[0] = 1
			right[0] = 1
		}

		f.irs[m] = [][]float64{left, right}
	}

	return f
}

// TestBinauralRenderPreservesSOFAITD is the end-to-end check: a SOFA file is
// loaded and driven through the real binaural render path, and the rendered
// left and right buffers must show the interaural time difference the file
// encodes, with the sign following the source's side.
func TestBinauralRenderPreservesSOFAITD(t *testing.T) {
	const itdSamples = 20

	dataset, err := Load(itdFixture(itdSamples).save(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	cfg := ir.RenderConfig{SampleRate: testSampleRate, DurationSeconds: 0.05}

	tests := []struct {
		name string
		// direction is in the receiver's head frame: +Y is left.
		direction geometry.Vec3
		wantLead  int // positive when the left ear leads
	}{
		{"source on the left", geometry.Vec3{Y: 1}, itdSamples},
		{"source on the right", geometry.Vec3{Y: -1}, -itdSamples},
		{"source straight ahead", geometry.Vec3{X: 1}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []ir.Event{{
				TimeSeconds:    0.001,
				Amplitude:      1,
				Direction:      tt.direction,
				DistanceMeters: 1,
				Kind:           ir.EventDirect,
			}}

			left, right, err := ir.RenderBinaural(events, *dataset, cfg)
			if err != nil {
				t.Fatalf("RenderBinaural() error = %v", err)
			}

			leftOnset := onsetIndex(t, left.Samples)
			rightOnset := onsetIndex(t, right.Samples)

			if got := rightOnset - leftOnset; got != tt.wantLead {
				t.Errorf("left ear leads by %d samples, want %d (left onset %d, right onset %d)",
					got, tt.wantLead, leftOnset, rightOnset)
			}
		})
	}
}

// onsetIndex returns the first sample whose magnitude passes a fraction of the
// buffer's peak, which locates the arrival without depending on its exact
// amplitude.
func onsetIndex(t *testing.T, samples []float64) int {
	t.Helper()

	var peak float64
	for _, s := range samples {
		peak = math.Max(peak, math.Abs(s))
	}

	if peak == 0 {
		t.Fatal("buffer is silent, so it has no onset")
	}

	for i, s := range samples {
		if math.Abs(s) >= peak/2 {
			return i
		}
	}

	t.Fatal("no sample reached half the peak")

	return -1
}
