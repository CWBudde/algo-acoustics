package metrics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestIACCIdenticalChannels(t *testing.T) {
	t.Parallel()

	// Identical left and right channels → IACC = 1.0 (fully correlated).
	sampleRate := 48000
	samples := make([]float64, sampleRate/10) // 100 ms

	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / float64(sampleRate))
	}

	left := &ir.Buffer{SampleRate: sampleRate, Samples: samples}
	right := &ir.Buffer{SampleRate: sampleRate, Samples: append([]float64{}, samples...)}

	iacc, err := IACC(left, right)
	if err != nil {
		t.Fatalf("IACC() error: %v", err)
	}

	if math.Abs(iacc-1.0) > 0.01 {
		t.Errorf("IACC = %.4f, want ~1.0 for identical channels", iacc)
	}
}

func TestIACCUncorrelatedChannels(t *testing.T) {
	t.Parallel()

	// Uncorrelated noise → IACC ≈ 0.
	sampleRate := 48000
	n := sampleRate // 1 second

	left := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, n)}
	right := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, n)}

	// Two different sine waves at unrelated frequencies.
	for i := range n {
		left.Samples[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / float64(sampleRate))
		right.Samples[i] = math.Sin(2 * math.Pi * 1373 * float64(i) / float64(sampleRate))
	}

	iacc, err := IACC(left, right)
	if err != nil {
		t.Fatalf("IACC() error: %v", err)
	}

	if iacc > 0.2 {
		t.Errorf("IACC = %.4f, want < 0.2 for uncorrelated channels", iacc)
	}
}

func TestIACCDelayedChannel(t *testing.T) {
	t.Parallel()

	// Right channel delayed by a small amount within ±1ms → IACC should still be ~1.0.
	sampleRate := 48000
	delaySamples := 20 // ~0.4 ms at 48 kHz, within ±1 ms window
	n := sampleRate / 10

	samples := make([]float64, n)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * 500 * float64(i) / float64(sampleRate))
	}

	left := &ir.Buffer{SampleRate: sampleRate, Samples: append([]float64{}, samples...)}

	rightSamples := make([]float64, n)
	for i := delaySamples; i < n; i++ {
		rightSamples[i] = samples[i-delaySamples]
	}

	right := &ir.Buffer{SampleRate: sampleRate, Samples: rightSamples}

	iacc, err := IACC(left, right)
	if err != nil {
		t.Fatalf("IACC() error: %v", err)
	}

	if iacc < 0.9 {
		t.Errorf("IACC = %.4f, want > 0.9 for delayed copy within 1ms", iacc)
	}
}

func TestIACCNilBuffers(t *testing.T) {
	t.Parallel()

	_, err := IACC(nil, nil)
	if err == nil {
		t.Error("expected error for nil buffers")
	}
}

func TestIACCEmptyBuffers(t *testing.T) {
	t.Parallel()

	left := &ir.Buffer{SampleRate: 48000, Samples: []float64{}}
	right := &ir.Buffer{SampleRate: 48000, Samples: []float64{}}

	_, err := IACC(left, right)
	if err == nil {
		t.Error("expected error for empty buffers")
	}
}

func TestIACCRange(t *testing.T) {
	t.Parallel()

	// IACC should always be in [0, 1].
	sampleRate := 48000
	n := sampleRate / 10

	left := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, n)}
	right := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, n)}

	for i := range n {
		left.Samples[i] = math.Sin(2 * math.Pi * 800 * float64(i) / float64(sampleRate))
		right.Samples[i] = -left.Samples[i] // anti-correlated
	}

	iacc, err := IACC(left, right)
	if err != nil {
		t.Fatalf("IACC() error: %v", err)
	}

	if iacc < 0 || iacc > 1 {
		t.Errorf("IACC = %.4f, want in [0, 1]", iacc)
	}

	// Anti-correlated should still have high IACC (absolute value).
	if iacc < 0.9 {
		t.Errorf("IACC = %.4f, want > 0.9 for anti-correlated channels", iacc)
	}
}
