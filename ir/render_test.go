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

func TestRenderMonoAppliesBandSumAndPhase(t *testing.T) {
	t.Parallel()

	buf, err := RenderMono([]Event{
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
	}, RenderConfig{
		SampleRate:      100,
		DurationSeconds: 0.1,
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{125, 250, 500},
			LowerEdges:  []float64{88, 177, 354},
			UpperEdges:  []float64{177, 354, 707},
		},
	})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if got, want := buf.Samples[1], 0.2916666666666667; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[1] = %v, want %v", got, want)
	}

	if got, want := buf.Samples[2], -0.4; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[2] = %v, want %v", got, want)
	}

	if got, want := buf.Samples[3], -0.35; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[3] = %v, want %v", got, want)
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
