package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func TestRenderHybridBandSumsEarlyAndLate(t *testing.T) {
	t.Parallel()

	early := []Event{{TimeSeconds: 0.01, Amplitude: 1, BandGain: []float64{1, 1}}}
	late := []Event{{TimeSeconds: 0.02, Amplitude: 2, BandGain: []float64{1, 1}}}

	buf, err := RenderHybridBand(early, late, 0, RenderConfig{
		SampleRate:      100,
		DurationSeconds: 0.1,
		BandSpec:        acoustics.BandSpec{CenterFreqs: []float64{125, 250}, LowerEdges: []float64{88, 177}, UpperEdges: []float64{177, 354}},
	})
	if err != nil {
		t.Fatalf("RenderHybridBand() error = %v", err)
	}

	if got, want := buf.Samples[1], 1.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[1] = %v, want %v", got, want)
	}

	if got, want := buf.Samples[2], 2.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[2] = %v, want %v", got, want)
	}
}

func TestSumBandsWeighted(t *testing.T) {
	t.Parallel()

	result := SumBandsWeighted([]*Buffer{
		{SampleRate: 100, Samples: []float64{1, 1}},
		{SampleRate: 100, Samples: []float64{2, 2}},
	}, []float64{0.5, 2})
	if result == nil {
		t.Fatal("SumBandsWeighted() = nil")
	}

	if got, want := result.Samples[0], 4.5; math.Abs(got-want) > 1e-12 {
		t.Fatalf("weighted sum = %v, want %v", got, want)
	}
}
