package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

// TestBlendLowFreqIdenticalInputsRoundtrip verifies the spectral complement
// identity: because wHigh = 1 − wLow at every frequency bin, blending two
// identical signals must return the original unchanged. Any deviation would
// mean the weights do not sum to 1.
func TestBlendLowFreqIdenticalInputsRoundtrip(t *testing.T) {
	t.Parallel()

	const n = 512
	signal := make([]float64, n)
	signal[0] = 1.0 // unit impulse at t = 0

	geoIR := &ir.Buffer{SampleRate: 4096, Samples: make([]float64, n)}
	copy(geoIR.Samples, signal)

	out := BlendLowFreq(signal, geoIR, 200, 4096)
	if out == nil {
		t.Fatal("BlendLowFreq() returned nil for identical inputs")
	}

	if len(out.Samples) == 0 {
		t.Fatal("BlendLowFreq() returned empty buffer for identical inputs")
	}

	// The impulse at sample 0 must survive the FFT round-trip.
	if math.Abs(out.Samples[0]-1.0) > 1e-6 {
		t.Fatalf("Samples[0] = %v, want 1.0 (identical inputs must round-trip through blend)", out.Samples[0])
	}

	// All other samples should be near zero (no spurious energy).
	for i := 1; i < min(n, len(out.Samples)); i++ {
		if math.Abs(out.Samples[i]) > 1e-6 {
			t.Fatalf("Samples[%d] = %v, want ~0 (impulse tail leak after round-trip)", i, out.Samples[i])
		}
	}
}

// TestBlendLowFreqZeroGeoPassesLowIR verifies that when the geometric IR is
// all zeros, the output retains contribution from the low-frequency signal.
// This confirms the low-pass branch carries energy rather than being silenced.
func TestBlendLowFreqZeroGeoPassesLowIR(t *testing.T) {
	t.Parallel()

	const n = 512
	lowIR := make([]float64, n)
	lowIR[0] = 1.0

	geoIR := &ir.Buffer{SampleRate: 4096, Samples: make([]float64, n)} // all zeros

	out := BlendLowFreq(lowIR, geoIR, 200, 4096)
	if out == nil {
		t.Fatal("BlendLowFreq() returned nil")
	}

	// The low-frequency impulse energy should appear in the output.
	if math.Abs(out.Samples[0]) < 1e-6 {
		t.Fatalf("Samples[0] = %v, want non-zero (lowpass of low-IR impulse)", out.Samples[0])
	}
}

// TestBlendLowFreqZeroLowPassesGeoIR verifies that when the low IR is all
// zeros, the output retains contribution from the geometric (high-frequency)
// signal.  This confirms the high-pass branch carries energy.
func TestBlendLowFreqZeroLowPassesGeoIR(t *testing.T) {
	t.Parallel()

	const n = 512
	lowIR := make([]float64, n) // all zeros

	geoIR := &ir.Buffer{SampleRate: 4096, Samples: make([]float64, n)}
	geoIR.Samples[0] = 1.0

	out := BlendLowFreq(lowIR, geoIR, 200, 4096)
	if out == nil {
		t.Fatal("BlendLowFreq() returned nil")
	}

	// The geometric impulse energy should appear in the output.
	if math.Abs(out.Samples[0]) < 1e-6 {
		t.Fatalf("Samples[0] = %v, want non-zero (highpass of geo-IR impulse)", out.Samples[0])
	}
}

func TestBlendLowFreqLowOnlySinusoidPreservesAmplitudeAndSymmetry(t *testing.T) {
	t.Parallel()

	const (
		n          = 1024
		sampleRate = 4096
		frequency  = 64
		crossover  = 1024
	)

	lowIR := make([]float64, n)
	for i := range lowIR {
		lowIR[i] = math.Sin(2 * math.Pi * frequency * float64(i) / sampleRate)
	}

	geoIR := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, n)}

	out := BlendLowFreq(lowIR, geoIR, crossover, sampleRate)
	if out == nil {
		t.Fatal("BlendLowFreq() returned nil")
	}

	var inputPower, outputPower, errorPower float64

	for i, input := range lowIR {
		output := out.Samples[i]
		inputPower += input * input
		outputPower += output * output
		difference := output - input
		errorPower += difference * difference
	}

	amplitudeRatio := math.Sqrt(outputPower / inputPower)
	if math.Abs(amplitudeRatio-1) > 1e-3 {
		t.Fatalf("output/input RMS amplitude ratio = %v, want 1", amplitudeRatio)
	}

	if relativeError := math.Sqrt(errorPower / inputPower); relativeError > 1e-3 {
		t.Fatalf("relative waveform error = %v, want <= 0.001; conjugate bins were not weighted symmetrically", relativeError)
	}
}
