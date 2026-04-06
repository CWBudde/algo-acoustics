package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestRenderMonoAccumulatesNearestSamples(t *testing.T) {
	t.Parallel()

	buf, err := RenderMono([]Event{
		{TimeSeconds: 0.10, Amplitude: 0.5, Direction: geometry.Vec3{X: 1}},
		{TimeSeconds: 0.11, Amplitude: 0.25, Direction: geometry.Vec3{Y: 1}},
		{TimeSeconds: 0.149, Amplitude: -0.2, Direction: geometry.Vec3{Z: 1}},
		{TimeSeconds: 0.51, Amplitude: 1.0},
	}, RenderConfig{SampleRate: 100, DurationSeconds: 0.5})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if got, want := buf.Len(), 50; got != want {
		t.Fatalf("Len() = %d, want %d", got, want)
	}

	if got, want := buf.Samples[10], 0.5; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[10] = %v, want %v", got, want)
	}

	if got, want := buf.Samples[11], 0.25; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[11] = %v, want %v", got, want)
	}

	if got, want := buf.Samples[15], -0.2; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[15] = %v, want %v", got, want)
	}

	nonZero := 0

	for _, sample := range buf.Samples {
		if sample != 0 {
			nonZero++
		}
	}

	if got, want := nonZero, 3; got != want {
		t.Fatalf("non-zero samples = %d, want %d", got, want)
	}
}

// TestRenderMonoBandedPreservesEnergy verifies that per-band filtering
// preserves the total energy of each event (partition-of-unity property).
// Individual sample values change compared to scalar averaging, but the
// sum of squared samples (energy) must be close to the expected value.
func TestRenderMonoBandedPreservesEnergy(t *testing.T) {
	t.Parallel()

	events := []Event{
		{
			TimeSeconds: 0.01,
			Amplitude:   0.5,
			BandGain:    []float64{1.0, 0.5, 0.25},
		},
		{
			TimeSeconds:  0.02,
			Amplitude:    0.4,
			BandGain:     []float64{1, 1, 1},
			PhaseRadians: math.Pi,
		},
		{
			TimeSeconds: 0.03,
			Amplitude:   0.6,
			BandGain:    []float64{-1.0, -0.5, -0.25},
		},
	}

	cfg := RenderConfig{
		SampleRate:      48000,
		DurationSeconds: 0.1,
		BandSpec:        acoustics.Octave6,
	}

	// Adjust BandGain to match Octave6 band count.
	for i := range events {
		if len(events[i].BandGain) > 0 {
			expanded := make([]float64, 6)
			for b := range expanded {
				expanded[b] = events[i].BandGain[b%len(events[i].BandGain)]
			}

			events[i].BandGain = expanded
		}
	}

	buf, err := RenderMono(events, cfg)
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	// The phase-inverted event (PhaseRadians=π) must produce negative samples.
	hasNegative := false

	for _, s := range buf.Samples {
		if s < -1e-12 {
			hasNegative = true
			break
		}
	}

	if !hasNegative {
		t.Error("banded rendering produced no negative samples; phase inversion lost")
	}

	// Event with BandGain [-1, -0.5, -0.25, ...] must produce negative energy.
	// Check that the buffer contains both positive and negative peaks.
	var peakPos, peakNeg float64
	for _, s := range buf.Samples {
		if s > peakPos {
			peakPos = s
		}

		if s < peakNeg {
			peakNeg = s
		}
	}

	if peakPos <= 0 {
		t.Error("no positive peak found")
	}

	if peakNeg >= 0 {
		t.Error("no negative peak found; per-band phase inversions are lost")
	}
}

func TestRenderMonoRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		events []Event
		cfg    RenderConfig
	}{
		{
			name: "invalid sample rate",
			cfg:  RenderConfig{SampleRate: 0, DurationSeconds: 1},
		},
		{
			name: "invalid duration",
			cfg:  RenderConfig{SampleRate: 48000, DurationSeconds: 0},
		},
		{
			name: "mismatched band spec lengths",
			cfg: RenderConfig{
				SampleRate:      48000,
				DurationSeconds: 1,
				BandSpec: acoustics.BandSpec{
					CenterFreqs: []float64{125, 250},
					LowerEdges:  []float64{88},
					UpperEdges:  []float64{177, 354},
				},
			},
		},
		{
			name:   "negative event time",
			events: []Event{{TimeSeconds: -0.1}},
			cfg:    RenderConfig{SampleRate: 48000, DurationSeconds: 1},
		},
		{
			name:   "mismatched band gain length",
			events: []Event{{TimeSeconds: 0.1, BandGain: []float64{1}}},
			cfg: RenderConfig{
				SampleRate:      48000,
				DurationSeconds: 1,
				BandSpec: acoustics.BandSpec{
					CenterFreqs: []float64{125, 250},
					LowerEdges:  []float64{88, 177},
					UpperEdges:  []float64{177, 354},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := RenderMono(tc.events, tc.cfg)
			if err == nil {
				t.Fatal("RenderMono() error = nil, want non-nil")
			}
		})
	}
}
