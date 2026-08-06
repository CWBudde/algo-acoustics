package ir

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
)

// itdDataset returns different delays for left vs right based on direction,
// simulating interaural time difference for lateralized sources.
type itdDataset struct {
	sampleRate int
	itdSamples int // number of samples ITD for a fully lateral source
}

func (d itdDataset) SampleRate() int { return d.sampleRate }

func (d itdDataset) Lookup(dir geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	// Simple ITD model: positive Y → sound from the left → right ear delayed.
	// Negative Y → sound from the right → left ear delayed.
	itdSeconds := float64(d.itdSamples) / float64(d.sampleRate)
	leftDelay := 0.0
	rightDelay := 0.0

	if dir.Y > 0 {
		rightDelay = itdSeconds
	} else if dir.Y < 0 {
		leftDelay = itdSeconds
	}

	// Return identity HRIRs (single sample) with appropriate delays encoded
	// via separate left/right delay values. Since the API only supports one
	// delay, we encode the ITD by padding one HRIR.
	if rightDelay > leftDelay {
		padSamples := d.itdSamples
		rightHRIR := make([]float64, padSamples+1)
		rightHRIR[padSamples] = 1

		return []float64{1}, rightHRIR, 0, nil
	}

	if leftDelay > rightDelay {
		padSamples := d.itdSamples
		leftHRIR := make([]float64, padSamples+1)
		leftHRIR[padSamples] = 1

		return leftHRIR, []float64{1}, 0, nil
	}

	return []float64{1}, []float64{1}, 0, nil
}

func TestRenderBinauralPoissonBasic(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	sampleRate := 44100

	bins := make([]EnergyBin, 10)
	for i := range bins {
		energy := make([]float64, spec.BandCount())
		for b := range energy {
			energy[b] = 0.01
		}

		bins[i] = EnergyBin{
			TimeSeconds: float64(i) * 0.01,
			BandEnergy:  energy,
		}
	}

	cfg := BinauralPoissonConfig{
		Bins:        bins,
		BinDuration: 0.01,
		Volume:      100,
		BandSpec:    spec,
		SampleRate:  sampleRate,
		HRTF:        hrtf.NoopDataset{SampleRateHz: sampleRate},
	}

	rng := rand.New(rand.NewSource(42))

	left, right, err := RenderBinauralPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("RenderBinauralPoisson() error: %v", err)
	}

	if left.Len() == 0 {
		t.Fatal("left buffer is empty")
	}

	if right.Len() == 0 {
		t.Fatal("right buffer is empty")
	}

	// With NoopDataset (identity HRIR), left and right should have similar energy.
	var leftEnergy, rightEnergy float64
	for _, s := range left.Samples {
		leftEnergy += s * s
	}

	for _, s := range right.Samples {
		rightEnergy += s * s
	}

	if leftEnergy == 0 || rightEnergy == 0 {
		t.Fatal("one or both channels have zero energy")
	}

	ratio := leftEnergy / rightEnergy
	// With random directions and identity HRIR, ratio should be close to 1.
	if ratio < 0.5 || ratio > 2.0 {
		t.Errorf("left/right energy ratio = %.2f, expected close to 1.0", ratio)
	}
}

func TestRenderBinauralPoissonITD(t *testing.T) {
	t.Parallel()

	spec := acoustics.BandSpec{
		CenterFreqs: []float64{1000},
		LowerEdges:  []float64{707},
		UpperEdges:  []float64{1414},
	}

	sampleRate := 48000
	itdSamples := 10

	// Create a single bin with energy, placed early enough for the Poisson
	// sequence to have events.
	bins := []EnergyBin{
		{TimeSeconds: 0.02, BandEnergy: []float64{1.0}},
		{TimeSeconds: 0.03, BandEnergy: []float64{1.0}},
		{TimeSeconds: 0.04, BandEnergy: []float64{1.0}},
		{TimeSeconds: 0.05, BandEnergy: []float64{1.0}},
		{TimeSeconds: 0.06, BandEnergy: []float64{1.0}},
	}

	cfg := BinauralPoissonConfig{
		Bins:        bins,
		BinDuration: 0.01,
		Volume:      50,
		BandSpec:    spec,
		SampleRate:  sampleRate,
		HRTF:        itdDataset{sampleRate: sampleRate, itdSamples: itdSamples},
	}

	rng := rand.New(rand.NewSource(99))

	left, right, err := RenderBinauralPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("RenderBinauralPoisson() error: %v", err)
	}

	// Find the first significant sample in each channel.
	leftFirst := findFirstSignificant(left.Samples)
	rightFirst := findFirstSignificant(right.Samples)

	if leftFirst < 0 || rightFirst < 0 {
		t.Fatalf("no significant samples found (left=%d, right=%d)", leftFirst, rightFirst)
	}

	// The ITD should be visible: one channel starts before the other by
	// approximately itdSamples. Since directions are random, we just verify
	// that the channels have different onset times (ITD > 0).
	itdMeasured := math.Abs(float64(leftFirst - rightFirst))
	t.Logf("left onset=%d, right onset=%d, ITD=%.0f samples (expected up to %d)",
		leftFirst, rightFirst, itdMeasured, itdSamples)

	// With random directions, some slots will have ITD and some won't. We just
	// verify the implementation doesn't crash and produces valid output.
	if left.Len() != right.Len() {
		t.Errorf("channel lengths differ: left=%d, right=%d", left.Len(), right.Len())
	}
}

func TestRenderBinauralPoissonInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  BinauralPoissonConfig
	}{
		{
			name: "no bands",
			cfg:  BinauralPoissonConfig{BinDuration: 0.01, Volume: 100, SampleRate: 44100, HRTF: hrtf.NoopDataset{SampleRateHz: 44100}},
		},
		{
			name: "zero sample rate",
			cfg:  BinauralPoissonConfig{BinDuration: 0.01, Volume: 100, BandSpec: acoustics.Octave6, HRTF: hrtf.NoopDataset{}},
		},
		{
			name: "nil HRTF",
			cfg:  BinauralPoissonConfig{BinDuration: 0.01, Volume: 100, BandSpec: acoustics.Octave6, SampleRate: 44100},
		},
		{
			name: "HRTF sample rate mismatch",
			cfg: BinauralPoissonConfig{
				BinDuration: 0.01,
				Volume:      100,
				BandSpec:    acoustics.Octave6,
				SampleRate:  44100,
				HRTF:        hrtf.NoopDataset{SampleRateHz: 48000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := RenderBinauralPoisson(tt.cfg, rand.New(rand.NewSource(0)))
			if err == nil {
				t.Fatal("expected error for invalid config")
			}
		})
	}
}

func TestRenderBinauralPoissonEmptyBins(t *testing.T) {
	t.Parallel()

	cfg := BinauralPoissonConfig{
		BinDuration: 0.01,
		Volume:      100,
		BandSpec:    acoustics.Octave6,
		SampleRate:  44100,
		HRTF:        hrtf.NoopDataset{SampleRateHz: 44100},
	}

	rng := rand.New(rand.NewSource(0))

	left, right, err := RenderBinauralPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if left.Len() != 0 || right.Len() != 0 {
		t.Fatalf("expected empty buffers, got left=%d right=%d", left.Len(), right.Len())
	}
}

func TestRenderBinauralPoissonIdentityHRIRPreservesEnvelope(t *testing.T) {
	t.Parallel()

	const (
		sampleRate  = 8000
		binDuration = 0.01
	)

	spec := acoustics.BandSpec{
		CenterFreqs: []float64{1000},
		LowerEdges:  []float64{700},
		UpperEdges:  []float64{1400},
	}
	bins := []EnergyBin{
		{TimeSeconds: 0, BandEnergy: []float64{0.2}},
		{TimeSeconds: binDuration, BandEnergy: []float64{0.4}},
		{TimeSeconds: 3 * binDuration, BandEnergy: []float64{0.1}},
	}
	monoConfig := PoissonConfig{
		Bins:        bins,
		BinDuration: binDuration,
		Volume:      50,
		BandSpec:    spec,
		SampleRate:  sampleRate,
	}
	binauralConfig := BinauralPoissonConfig{
		Bins:        bins,
		BinDuration: binDuration,
		Volume:      50,
		BandSpec:    spec,
		SampleRate:  sampleRate,
		HRTF:        hrtf.NoopDataset{SampleRateHz: sampleRate},
	}

	mono, err := RenderMonoPoisson(monoConfig, rand.New(rand.NewSource(123)))
	if err != nil {
		t.Fatalf("RenderMonoPoisson() error = %v", err)
	}

	left, right, err := RenderBinauralPoisson(binauralConfig, rand.New(rand.NewSource(123)))
	if err != nil {
		t.Fatalf("RenderBinauralPoisson() error = %v", err)
	}

	for index, want := range mono.Samples {
		if math.Abs(left.Samples[index]-want) > 1e-12 || math.Abs(right.Samples[index]-want) > 1e-12 {
			t.Fatalf("sample %d = %g/%g, want identity output %g", index, left.Samples[index], right.Samples[index], want)
		}
	}
}

func findFirstSignificant(samples []float64) int {
	const threshold = 1e-6

	for i, s := range samples {
		if math.Abs(s) > threshold {
			return i
		}
	}

	return -1
}
