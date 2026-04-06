package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	algofft "github.com/cwbudde/algo-fft"
)

// TestBandpassWeightsPartitionOfUnity verifies that the bandpass weights sum
// to 1.0 at every frequency bin. This is the core correctness property of the
// crossover design: no energy is created or lost.
func TestBandpassWeightsPartitionOfUnity(t *testing.T) {
	t.Parallel()

	const (
		fftSize    = 1024
		sampleRate = 48000
	)

	specs := []struct {
		name string
		spec acoustics.BandSpec
	}{
		{"Octave6", acoustics.Octave6},
		{"Octave8", acoustics.Octave8},
		{"single-band", acoustics.BandSpec{
			CenterFreqs: []float64{500},
			LowerEdges:  []float64{354},
			UpperEdges:  []float64{707},
		}},
	}

	for _, tc := range specs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			weights := buildBandpassWeights(tc.spec, fftSize, sampleRate)
			bandCount := tc.spec.BandCount()

			for k := range fftSize {
				sum := 0.0
				for b := range bandCount {
					sum += weights[b][k]
				}

				if math.Abs(sum-1.0) > 1e-12 {
					freq := float64(k) * float64(sampleRate) / float64(fftSize)

					t.Fatalf("bin %d (%.1f Hz): weight sum = %v, want 1.0", k, freq, sum)
				}
			}
		})
	}
}

// TestBandpassWeightsNonNegative verifies that no bandpass weight is negative.
func TestBandpassWeightsNonNegative(t *testing.T) {
	t.Parallel()

	weights := buildBandpassWeights(acoustics.Octave6, 1024, 48000)

	for b, bw := range weights {
		for k, w := range bw {
			if w < 0 {
				t.Fatalf("band %d, bin %d: weight = %v, must be non-negative", b, k, w)
			}
		}
	}
}

// TestAssignBandWeightsEmptySpec does nothing for zero bands.
func TestAssignBandWeightsEmptySpec(t *testing.T) {
	t.Parallel()

	spec := acoustics.BandSpec{}
	weights := make([][]float64, 0)

	assignBandWeights(weights, 0, 100, spec)
	// No panic, no assignment — pass.
}

// TestLogRatioEdgeCases verifies clamping at boundaries.
func TestLogRatioEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		freq     float64
		low      float64
		high     float64
		expected float64
	}{
		{"at low", 100, 100, 400, 0},
		{"below low", 50, 100, 400, 0},
		{"at high", 400, 100, 400, 1},
		{"above high", 800, 100, 400, 1},
		{"midpoint", 200, 100, 400, 0.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := logRatio(tc.freq, tc.low, tc.high)

			if math.Abs(got-tc.expected) > 1e-12 {
				t.Fatalf("logRatio(%v, %v, %v) = %v, want %v",
					tc.freq, tc.low, tc.high, got, tc.expected)
			}
		})
	}
}

// TestNextPow2 verifies power-of-two rounding.
func TestNextPow2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{1023, 1024},
		{1024, 1024},
		{1025, 2048},
	}

	for _, tc := range tests {
		got := nextPow2(tc.input)

		if got != tc.expected {
			t.Fatalf("nextPow2(%d) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

// TestHasBandedEvents verifies detection of banded vs wideband event lists.
func TestHasBandedEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		events   []Event
		expected bool
	}{
		{"empty", nil, false},
		{"wideband only", []Event{{Amplitude: 1}}, false},
		{"banded", []Event{{Amplitude: 1, BandGain: []float64{1, 0.5}}}, true},
		{"mixed", []Event{
			{Amplitude: 1},
			{Amplitude: 0.5, BandGain: []float64{1}},
		}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := hasBandedEvents(tc.events)

			if got != tc.expected {
				t.Fatalf("hasBandedEvents() = %v, want %v", got, tc.expected)
			}
		})
	}
}

// TestRenderMonoBandedWidebandOnlyMatchesScalar verifies that wideband events
// (no BandGain) rendered through the banded FFT path produce the same result
// as the scalar path. This exercises accumulateWidebandSpectrum in isolation.
func TestRenderMonoBandedWidebandOnlyMatchesScalar(t *testing.T) {
	t.Parallel()

	events := []Event{
		{TimeSeconds: 0.01, Amplitude: 1.0},
		{TimeSeconds: 0.02, Amplitude: 0.5, PhaseRadians: math.Pi / 4},
		{TimeSeconds: 0.03, Amplitude: 0.25},
	}

	cfg := RenderConfig{
		SampleRate:      1000,
		DurationSeconds: 0.1,
	}

	scalarBuf, err := renderMonoScalar(events, cfg)
	if err != nil {
		t.Fatalf("renderMonoScalar() error = %v", err)
	}

	// Force banded path by adding a BandSpec and a dummy banded event,
	// then also include the wideband events.
	bandedEvents := make([]Event, 0, len(events)+1)
	bandedEvents = append(bandedEvents, events...)

	bandedCfg := RenderConfig{
		SampleRate:      1000,
		DurationSeconds: 0.1,
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{125, 500},
			LowerEdges:  []float64{88, 354},
			UpperEdges:  []float64{177, 707},
		},
	}

	// Add a zero-amplitude banded event to trigger the banded path
	// without contributing energy.
	bandedEvents = append(bandedEvents, Event{
		TimeSeconds: 0.05,
		Amplitude:   0,
		BandGain:    []float64{1, 1},
	})

	bandedBuf, err := renderMonoBanded(bandedEvents, bandedCfg)
	if err != nil {
		t.Fatalf("renderMonoBanded() error = %v", err)
	}

	// Both buffers should have the same sample values.
	for i := range scalarBuf.Samples {
		if math.Abs(scalarBuf.Samples[i]-bandedBuf.Samples[i]) > 1e-10 {
			t.Fatalf("sample %d: scalar=%v, banded=%v",
				i, scalarBuf.Samples[i], bandedBuf.Samples[i])
		}
	}
}

// TestRenderMonoBandedPhaseInversion verifies that PhaseRadians=π through the
// banded FFT path negates the output, matching cos(π) = -1.
func TestRenderMonoBandedPhaseInversion(t *testing.T) {
	t.Parallel()

	spec := acoustics.BandSpec{
		CenterFreqs: []float64{500},
		LowerEdges:  []float64{354},
		UpperEdges:  []float64{707},
	}

	normal, err := renderMonoBanded([]Event{
		{TimeSeconds: 0.01, Amplitude: 1.0, BandGain: []float64{1.0}},
	}, RenderConfig{SampleRate: 48000, DurationSeconds: 0.05, BandSpec: spec})
	if err != nil {
		t.Fatalf("normal render error = %v", err)
	}

	inverted, err := renderMonoBanded([]Event{
		{TimeSeconds: 0.01, Amplitude: 1.0, BandGain: []float64{1.0}, PhaseRadians: math.Pi},
	}, RenderConfig{SampleRate: 48000, DurationSeconds: 0.05, BandSpec: spec})
	if err != nil {
		t.Fatalf("inverted render error = %v", err)
	}

	// Every sample should be negated.
	for i := range normal.Samples {
		if math.Abs(normal.Samples[i]+inverted.Samples[i]) > 1e-10 {
			t.Fatalf("sample %d: normal=%v, inverted=%v (should be negated)",
				i, normal.Samples[i], inverted.Samples[i])
		}
	}
}

// TestRenderMonoBandedMixedEvents verifies that banded and wideband events
// both contribute when mixed in the same render call.
func TestRenderMonoBandedMixedEvents(t *testing.T) {
	t.Parallel()

	spec := acoustics.BandSpec{
		CenterFreqs: []float64{500},
		LowerEdges:  []float64{354},
		UpperEdges:  []float64{707},
	}

	cfg := RenderConfig{
		SampleRate:      48000,
		DurationSeconds: 0.05,
		BandSpec:        spec,
	}

	// Render with only the banded event.
	bandedOnly, err := renderMonoBanded([]Event{
		{TimeSeconds: 0.01, Amplitude: 1.0, BandGain: []float64{1.0}},
	}, cfg)
	if err != nil {
		t.Fatalf("banded-only error = %v", err)
	}

	// Render with both banded and wideband.
	mixed, err := renderMonoBanded([]Event{
		{TimeSeconds: 0.01, Amplitude: 1.0, BandGain: []float64{1.0}},
		{TimeSeconds: 0.02, Amplitude: 0.5},
	}, cfg)
	if err != nil {
		t.Fatalf("mixed error = %v", err)
	}

	// The mixed result must differ from banded-only (wideband contributed).
	differs := false

	for i := range mixed.Samples {
		if math.Abs(mixed.Samples[i]-bandedOnly.Samples[i]) > 1e-12 {
			differs = true

			break
		}
	}

	if !differs {
		t.Fatal("mixed render identical to banded-only; wideband event had no effect")
	}

	// The wideband event at t=0.02 should add energy at sample 960.
	widebandSample := 960 // 0.02 * 48000
	diff := mixed.Samples[widebandSample] - bandedOnly.Samples[widebandSample]

	if math.Abs(diff-0.5) > 1e-10 {
		t.Fatalf("wideband contribution at sample %d = %v, want ~0.5", widebandSample, diff)
	}
}

// TestRenderMonoBandedSingleBandEnergyLocalization verifies that an event with
// gain only in one band produces energy concentrated in that band's frequency
// range and near-zero energy outside it.
func TestRenderMonoBandedSingleBandEnergyLocalization(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	cfg := RenderConfig{
		SampleRate:      48000,
		DurationSeconds: 0.05,
		BandSpec:        spec,
	}

	// Put all energy in band 2 (500 Hz center, ~354–707 Hz).
	gains := make([]float64, 6)
	gains[2] = 1.0

	buf, err := renderMonoBanded([]Event{
		{TimeSeconds: 0.01, Amplitude: 1.0, BandGain: gains},
	}, cfg)
	if err != nil {
		t.Fatalf("render error = %v", err)
	}

	// Compute energy via FFT to check frequency localization.
	fftSize := nextPow2(2 * buf.Len())
	input := make([]complex128, fftSize)

	for i, s := range buf.Samples {
		input[i] = complex(s, 0)
	}

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		t.Fatalf("FFT plan error = %v", err)
	}

	spectrum := make([]complex128, fftSize)

	err = plan.Forward(spectrum, input)
	if err != nil {
		t.Fatalf("FFT error = %v", err)
	}

	// Measure energy inside vs outside the 500 Hz band (354–707 Hz).
	var energyInBand, energyOutside float64

	nyquist := float64(cfg.SampleRate) / 2

	for k := range fftSize / 2 {
		freq := float64(k) * float64(cfg.SampleRate) / float64(fftSize)
		mag2 := real(spectrum[k])*real(spectrum[k]) + imag(spectrum[k])*imag(spectrum[k])

		if freq > nyquist {
			continue
		}

		if freq >= 354 && freq <= 707 {
			energyInBand += mag2
		} else {
			energyOutside += mag2
		}
	}

	if energyInBand <= 0 {
		t.Fatal("no energy in target band")
	}

	// At least 90% of energy should be in the target band.
	ratio := energyInBand / (energyInBand + energyOutside)

	if ratio < 0.90 {
		t.Fatalf("energy in band = %.1f%%, want >= 90%%", ratio*100)
	}
}

// TestRenderMonoBandedPhaseCancellation verifies that two banded events at the
// same time with phases 0 and π cancel each other.
func TestRenderMonoBandedPhaseCancellation(t *testing.T) {
	t.Parallel()

	spec := acoustics.BandSpec{
		CenterFreqs: []float64{500},
		LowerEdges:  []float64{354},
		UpperEdges:  []float64{707},
	}

	buf, err := renderMonoBanded([]Event{
		{TimeSeconds: 0.01, Amplitude: 1.0, BandGain: []float64{1.0}, PhaseRadians: 0},
		{TimeSeconds: 0.01, Amplitude: 1.0, BandGain: []float64{1.0}, PhaseRadians: math.Pi},
	}, RenderConfig{SampleRate: 48000, DurationSeconds: 0.05, BandSpec: spec})
	if err != nil {
		t.Fatalf("render error = %v", err)
	}

	for i, s := range buf.Samples {
		if math.Abs(s) > 1e-10 {
			t.Fatalf("sample %d = %v after phase cancellation, want 0", i, s)
		}
	}
}

// TestValidateBandedEventsRejectsNegativeTime verifies validation catches
// negative event times.
func TestValidateBandedEventsRejectsNegativeTime(t *testing.T) {
	t.Parallel()

	err := validateBandedEvents([]Event{
		{TimeSeconds: -1},
	}, 3)
	if err == nil {
		t.Fatal("expected error for negative time")
	}
}

// TestValidateBandedEventsRejectsMismatchedBandGain verifies validation
// catches band gain length mismatches.
func TestValidateBandedEventsRejectsMismatchedBandGain(t *testing.T) {
	t.Parallel()

	err := validateBandedEvents([]Event{
		{TimeSeconds: 0.01, BandGain: []float64{1, 2}},
	}, 3)
	if err == nil {
		t.Fatal("expected error for mismatched band gain length")
	}
}

// TestValidateBandedEventsAcceptsValid verifies that valid events pass.
func TestValidateBandedEventsAcceptsValid(t *testing.T) {
	t.Parallel()

	err := validateBandedEvents([]Event{
		{TimeSeconds: 0.01, BandGain: []float64{1, 2, 3}},
		{TimeSeconds: 0.02}, // wideband, no BandGain — always OK
	}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
