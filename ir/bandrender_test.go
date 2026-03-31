package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func TestRenderBandUsesSelectedBandAndPhase(t *testing.T) {
	t.Parallel()

	buf, err := RenderBand([]Event{
		{
			TimeSeconds: 0.01,
			Amplitude:   0.5,
			BandGain:    []float64{1, 2, 3},
		},
		{
			TimeSeconds:  0.02,
			Amplitude:    0.25,
			BandGain:     []float64{1, 1, 1},
			PhaseRadians: math.Pi,
		},
		{
			TimeSeconds: 0.03,
			Amplitude:   0.4,
		},
	}, 1, RenderConfig{
		SampleRate:      100,
		DurationSeconds: 0.1,
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{125, 250, 500},
			LowerEdges:  []float64{88, 177, 354},
			UpperEdges:  []float64{177, 354, 707},
		},
	})
	if err != nil {
		t.Fatalf("RenderBand() error = %v", err)
	}

	if got, want := buf.Samples[1], 1.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[1] = %v, want %v", got, want)
	}
	if got, want := buf.Samples[2], -0.25; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[2] = %v, want %v", got, want)
	}
	if got, want := buf.Samples[3], 0.4; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[3] = %v, want %v", got, want)
	}
}

func TestRenderBandRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		events []Event
		band   int
		cfg    RenderConfig
	}{
		{
			name: "invalid band index",
			band: 3,
			cfg: RenderConfig{
				SampleRate:      48000,
				DurationSeconds: 1,
				BandSpec: acoustics.BandSpec{
					CenterFreqs: []float64{125, 250, 500},
					LowerEdges:  []float64{88, 177, 354},
					UpperEdges:  []float64{177, 354, 707},
				},
			},
		},
		{
			name:   "negative event time",
			events: []Event{{TimeSeconds: -0.1}},
			band:   0,
			cfg: RenderConfig{
				SampleRate:      48000,
				DurationSeconds: 1,
				BandSpec: acoustics.BandSpec{
					CenterFreqs: []float64{125},
					LowerEdges:  []float64{88},
					UpperEdges:  []float64{177},
				},
			},
		},
		{
			name:   "mismatched band gain length",
			events: []Event{{TimeSeconds: 0.1, BandGain: []float64{1}}},
			band:   0,
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
			if _, err := RenderBand(tc.events, tc.band, tc.cfg); err == nil {
				t.Fatal("RenderBand() error = nil, want non-nil")
			}
		})
	}
}

func TestSumBands(t *testing.T) {
	t.Parallel()

	result := SumBands([]*Buffer{
		{SampleRate: 48000, Samples: []float64{1, 2, 3}},
		nil,
		{SampleRate: 48000, Samples: []float64{0.5, -1, 0.25}},
	})
	if result == nil {
		t.Fatal("SumBands() = nil, want buffer")
	}

	if got, want := result.SampleRate, 48000; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}
	want := []float64{1.5, 1, 3.25}
	if len(result.Samples) != len(want) {
		t.Fatalf("len(Samples) = %d, want %d", len(result.Samples), len(want))
	}
	for index := range want {
		if got := result.Samples[index]; math.Abs(got-want[index]) > 1e-12 {
			t.Fatalf("Samples[%d] = %v, want %v", index, got, want[index])
		}
	}

	if got := SumBands(nil); got != nil {
		t.Fatalf("SumBands(nil) = %#v, want nil", got)
	}
	if got := SumBands([]*Buffer{nil, nil}); got != nil {
		t.Fatalf("SumBands(all nil) = %#v, want nil", got)
	}
}
