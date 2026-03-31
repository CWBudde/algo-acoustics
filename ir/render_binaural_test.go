package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
)

type directionalDataset struct{}

func (directionalDataset) SampleRate() int { return 100 }

func (directionalDataset) Lookup(dir geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	if dir.X >= 0 {
		return []float64{1}, []float64{0.5}, 0, nil
	}

	return []float64{0.5}, []float64{1}, 0, nil
}

func TestRenderBinauralWithNoopDatasetMatchesMono(t *testing.T) {
	t.Parallel()

	events := []Event{{TimeSeconds: 0.01, Amplitude: 0.5, Direction: geometry.Vec3{X: 1}}, {TimeSeconds: 0.02, Amplitude: 0.25, Direction: geometry.Vec3{Y: 1}}}

	mono, err := RenderMono(events, RenderConfig{SampleRate: 100, DurationSeconds: 0.1})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	left, right, err := RenderBinaural(events, hrtf.NoopDataset{SampleRateHz: 100}, RenderConfig{SampleRate: 100, DurationSeconds: 0.1})
	if err != nil {
		t.Fatalf("RenderBinaural() error = %v", err)
	}

	if left.Len() != mono.Len() || right.Len() != mono.Len() {
		t.Fatalf("stereo lengths = %d/%d, want %d", left.Len(), right.Len(), mono.Len())
	}

	for i := range mono.Samples {
		if math.Abs(left.Samples[i]-mono.Samples[i]) > 1e-12 {
			t.Fatalf("left[%d] = %v, want %v", i, left.Samples[i], mono.Samples[i])
		}

		if math.Abs(right.Samples[i]-mono.Samples[i]) > 1e-12 {
			t.Fatalf("right[%d] = %v, want %v", i, right.Samples[i], mono.Samples[i])
		}
	}
}

func TestRenderBinauralUsesDirectionalHRTF(t *testing.T) {
	t.Parallel()

	left, right, err := RenderBinaural([]Event{{TimeSeconds: 0.01, Amplitude: 1, Direction: geometry.Vec3{X: 1}}}, directionalDataset{}, RenderConfig{SampleRate: 100, DurationSeconds: 0.1, BandSpec: acoustics.BandSpec{CenterFreqs: []float64{125}, LowerEdges: []float64{88}, UpperEdges: []float64{177}}})
	if err != nil {
		t.Fatalf("RenderBinaural() error = %v", err)
	}

	if math.Abs(left.Samples[1]-1) > 1e-12 {
		t.Fatalf("left sample = %v, want 1", left.Samples[1])
	}

	if math.Abs(right.Samples[1]-0.5) > 1e-12 {
		t.Fatalf("right sample = %v, want 0.5", right.Samples[1])
	}
}
