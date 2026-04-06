package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

// TestRenderMonoSingleEventPeakEqualsAmplitude verifies that a single event
// with no band gains and zero phase produces exactly its amplitude at the
// correct sample index and nowhere else.
func TestRenderMonoSingleEventPeakEqualsAmplitude(t *testing.T) {
	t.Parallel()

	const amplitude = 0.75

	buf, err := RenderMono([]Event{
		{TimeSeconds: 0.1, Amplitude: amplitude},
	}, RenderConfig{SampleRate: 1000, DurationSeconds: 0.5})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if got, want := buf.Samples[100], amplitude; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[100] = %v, want %v", got, want)
	}

	peak := 0.0
	for _, s := range buf.Samples {
		if math.Abs(s) > math.Abs(peak) {
			peak = s
		}
	}

	if math.Abs(peak-amplitude) > 1e-12 {
		t.Fatalf("peak = %v, want %v (no other samples should have energy)", peak, amplitude)
	}
}

// TestRenderMonoPhasePiInvertsAmplitude verifies that cos(π) = -1 inverts the
// amplitude of an event with PhaseRadians = π.
func TestRenderMonoPhasePiInvertsAmplitude(t *testing.T) {
	t.Parallel()

	buf, err := RenderMono([]Event{
		{TimeSeconds: 0.1, Amplitude: 0.5, PhaseRadians: math.Pi},
	}, RenderConfig{SampleRate: 1000, DurationSeconds: 0.5})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if got, want := buf.Samples[100], -0.5; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Samples[100] = %v, want %v (PhaseRadians=π must negate amplitude)", got, want)
	}
}

// TestRenderMonoPhaseCancellationProducesSilence verifies that two identical
// events landing on the same sample with phases 0 and π cancel exactly.
func TestRenderMonoPhaseCancellationProducesSilence(t *testing.T) {
	t.Parallel()

	buf, err := RenderMono([]Event{
		{TimeSeconds: 0.1, Amplitude: 1.0, PhaseRadians: 0},
		{TimeSeconds: 0.1, Amplitude: 1.0, PhaseRadians: math.Pi},
	}, RenderConfig{SampleRate: 1000, DurationSeconds: 0.5})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	for i, s := range buf.Samples {
		if math.Abs(s) > 1e-12 {
			t.Fatalf("Samples[%d] = %v after full phase cancellation, want 0", i, s)
		}
	}
}

// TestRenderMonoEventBeyondDurationIsDropped verifies that events whose sample
// index equals or exceeds the buffer length are silently skipped, not errored.
func TestRenderMonoEventBeyondDurationIsDropped(t *testing.T) {
	t.Parallel()

	buf, err := RenderMono([]Event{
		{TimeSeconds: 0.05, Amplitude: 1.0},
		{TimeSeconds: 0.40, Amplitude: 2.0}, // rounds to sample 400 == Len → dropped
		{TimeSeconds: 0.50, Amplitude: 3.0}, // past duration → dropped
	}, RenderConfig{SampleRate: 1000, DurationSeconds: 0.4})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if got, want := buf.Len(), 400; got != want {
		t.Fatalf("Len() = %d, want %d", got, want)
	}

	nonZero := 0

	for _, s := range buf.Samples {
		if s != 0 {
			nonZero++
		}
	}

	if nonZero != 1 {
		t.Fatalf("non-zero samples = %d, want 1 (events at/past duration must be dropped)", nonZero)
	}
}

// TestRenderMonoRenderedGainsSumMatchesExpected verifies that when events land
// on distinct samples, the arithmetic sum of all rendered sample values equals
// the sum of per-event gains (amplitude × avgBandGain × cos(phase)).
func TestRenderMonoRenderedGainsSumMatchesExpected(t *testing.T) {
	t.Parallel()

	cfg := RenderConfig{
		SampleRate:      1000,
		DurationSeconds: 0.1,
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{125, 250, 500},
			LowerEdges:  []float64{88, 177, 354},
			UpperEdges:  []float64{177, 354, 707},
		},
	}
	events := []Event{
		{TimeSeconds: 0.010, Amplitude: 1.0},
		{TimeSeconds: 0.020, Amplitude: 0.5},
		{TimeSeconds: 0.030, Amplitude: 0.25, PhaseRadians: math.Pi / 3},
		{TimeSeconds: 0.040, Amplitude: 0.8, BandGain: []float64{0.6, 0.8, 1.0}},
	}

	buf, err := RenderMono(events, cfg)
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	// Expected per-event contributions:
	//   [0]: 1.0 × 1.0 × cos(0)     = 1.000
	//   [1]: 0.5 × 1.0 × cos(0)     = 0.500
	//   [2]: 0.25 × 1.0 × cos(π/3)  = 0.125  (cos(π/3) = 0.5)
	//   [3]: 0.8 × (0.6+0.8+1.0)/3  = 0.640  (avgBandGain = 0.8)
	wantSum := 1.0 + 0.5 + 0.25*math.Cos(math.Pi/3) + 0.8*(0.6+0.8+1.0)/3

	var gotSum float64
	for _, s := range buf.Samples {
		gotSum += s
	}

	if math.Abs(gotSum-wantSum) > 1e-12 {
		t.Fatalf("sum of rendered samples = %v, want %v", gotSum, wantSum)
	}
}

// TestRenderMonoBandGainAllZeroProducesSilence verifies that an event with all
// band gains explicitly set to zero contributes nothing to the buffer,
// regardless of amplitude.
func TestRenderMonoBandGainAllZeroProducesSilence(t *testing.T) {
	t.Parallel()

	buf, err := RenderMono([]Event{
		{TimeSeconds: 0.05, Amplitude: 100.0, BandGain: []float64{0, 0, 0}},
	}, RenderConfig{
		SampleRate:      1000,
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

	for i, s := range buf.Samples {
		if s != 0 {
			t.Fatalf("Samples[%d] = %v, want 0 (all band gains are zero)", i, s)
		}
	}
}
