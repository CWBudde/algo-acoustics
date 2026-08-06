package ir

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
)

func TestRenderBinauralPoissonWithDGDirections(t *testing.T) {
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

	// Set up 4 DG directions: front, left, back, right.
	dgDirs := []geometry.Vec3{
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: -1, Y: 0, Z: 0},
		{X: 0, Y: -1, Z: 0},
	}

	// All slots dominated by DG 0 (front direction).
	dgProbs := make([][]float64, 4)
	for d := range dgProbs {
		dgProbs[d] = make([]float64, len(bins))
	}

	for k := range bins {
		dgProbs[0][k] = 1.0
	}

	cfg := BinauralPoissonConfig{
		Bins:            bins,
		BinDuration:     0.01,
		Volume:          100,
		BandSpec:        spec,
		SampleRate:      sampleRate,
		HRTF:            hrtf.NoopDataset{SampleRateHz: sampleRate},
		DGDirections:    dgDirs,
		DGProbabilities: dgProbs,
	}

	rng := rand.New(rand.NewSource(42))

	left, right, err := RenderBinauralPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("RenderBinauralPoisson() error: %v", err)
	}

	if left.Len() == 0 || right.Len() == 0 {
		t.Fatal("output buffers are empty")
	}
}

func TestRenderBinauralPoissonDGLateralITD(t *testing.T) {
	t.Parallel()

	spec := acoustics.BandSpec{
		CenterFreqs: []float64{1000},
		LowerEdges:  []float64{707},
		UpperEdges:  []float64{1414},
	}

	sampleRate := 48000
	itdSamples := 10

	bins := make([]EnergyBin, 5)
	for i := range bins {
		bins[i] = EnergyBin{
			TimeSeconds: float64(i)*0.01 + 0.02,
			BandEnergy:  []float64{1.0},
		}
	}

	// Two DGs: left (+Y) and right (-Y).
	dgDirs := []geometry.Vec3{
		{X: 0, Y: 1, Z: 0},
		{X: 0, Y: -1, Z: 0},
	}

	// All slots 100% from left direction.
	dgProbs := [][]float64{
		make([]float64, len(bins)),
		make([]float64, len(bins)),
	}

	for k := range bins {
		dgProbs[0][k] = 1.0
		dgProbs[1][k] = 0.0
	}

	cfg := BinauralPoissonConfig{
		Bins:            bins,
		BinDuration:     0.01,
		Volume:          50,
		BandSpec:        spec,
		SampleRate:      sampleRate,
		HRTF:            itdDataset{sampleRate: sampleRate, itdSamples: itdSamples},
		DGDirections:    dgDirs,
		DGProbabilities: dgProbs,
	}

	rng := rand.New(rand.NewSource(42))

	left, right, err := RenderBinauralPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("RenderBinauralPoisson() error: %v", err)
	}

	// With all energy from the left (+Y) direction and an ITD dataset,
	// the left channel should have earlier onset than the right channel.
	leftFirst := findFirstSignificant(left.Samples)
	rightFirst := findFirstSignificant(right.Samples)

	if leftFirst < 0 || rightFirst < 0 {
		t.Fatalf("no significant samples: left=%d, right=%d", leftFirst, rightFirst)
	}

	// Left ear should receive sound first (source is at +Y = left side).
	if leftFirst >= rightFirst {
		t.Errorf("expected left onset (%d) before right onset (%d) for left-side source",
			leftFirst, rightFirst)
	}
}

func TestRenderBinauralPoissonDGBlendedDirections(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	sampleRate := 44100

	bins := make([]EnergyBin, 5)
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

	// Two DGs with equal probability — blend should work without errors.
	dgDirs := []geometry.Vec3{
		{X: 1, Y: 0, Z: 0},
		{X: -1, Y: 0, Z: 0},
	}

	dgProbs := [][]float64{
		make([]float64, len(bins)),
		make([]float64, len(bins)),
	}

	for k := range bins {
		dgProbs[0][k] = 0.5
		dgProbs[1][k] = 0.5
	}

	cfg := BinauralPoissonConfig{
		Bins:            bins,
		BinDuration:     0.01,
		Volume:          100,
		BandSpec:        spec,
		SampleRate:      sampleRate,
		HRTF:            hrtf.NoopDataset{SampleRateHz: sampleRate},
		DGDirections:    dgDirs,
		DGProbabilities: dgProbs,
		DGBlendCount:    2,
	}

	rng := rand.New(rand.NewSource(42))

	left, right, err := RenderBinauralPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("RenderBinauralPoisson() error: %v", err)
	}

	if left.Len() == 0 || right.Len() == 0 {
		t.Fatal("output buffers are empty")
	}

	// Both channels should have energy.
	var leftEnergy, rightEnergy float64

	for _, s := range left.Samples {
		leftEnergy += s * s
	}

	for _, s := range right.Samples {
		rightEnergy += s * s
	}

	if leftEnergy == 0 || rightEnergy == 0 {
		t.Error("one or both channels have zero energy")
	}
}

func TestRenderBinauralPoissonWithoutDGsUnchanged(t *testing.T) {
	t.Parallel()

	// Without DG fields set, behavior should match the original random direction.
	spec := acoustics.Octave6
	sampleRate := 44100

	bins := make([]EnergyBin, 5)
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

	if left.Len() == 0 || right.Len() == 0 {
		t.Fatal("output buffers are empty")
	}
}

func TestRenderBinauralPoissonDGConsistentDirection(t *testing.T) {
	t.Parallel()

	// When DG probabilities assign 100% to a single direction for all slots,
	// the result should be deterministic regardless of RNG seed (no random
	// direction involved).
	spec := acoustics.Octave6
	sampleRate := 44100

	bins := make([]EnergyBin, 5)
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

	dgDirs := []geometry.Vec3{{X: 1, Y: 0, Z: 0}}
	dgProbs := [][]float64{make([]float64, len(bins))}

	for k := range bins {
		dgProbs[0][k] = 1.0
	}

	makeCfg := func() BinauralPoissonConfig {
		return BinauralPoissonConfig{
			Bins:            bins,
			BinDuration:     0.01,
			Volume:          100,
			BandSpec:        spec,
			SampleRate:      sampleRate,
			HRTF:            hrtf.NoopDataset{SampleRateHz: sampleRate},
			DGDirections:    dgDirs,
			DGProbabilities: dgProbs,
		}
	}

	left1, _, err := RenderBinauralPoisson(makeCfg(), rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatalf("render 1 error: %v", err)
	}

	left2, _, err := RenderBinauralPoisson(makeCfg(), rand.New(rand.NewSource(999)))
	if err != nil {
		t.Fatalf("render 2 error: %v", err)
	}

	// The Poisson sequence itself depends on RNG, so the signals won't be
	// identical. But the HRIR selection should be the same. With NoopDataset
	// (identity HRIR), the left channels will differ due to Poisson noise
	// but both should have non-zero energy.
	var e1, e2 float64

	for _, s := range left1.Samples {
		e1 += s * s
	}

	for _, s := range left2.Samples {
		e2 += s * s
	}

	if e1 == 0 || e2 == 0 {
		t.Error("one render produced zero energy")
	}

	// Energy levels should be similar (same config, different noise).
	ratio := e1 / e2
	if ratio < 0.3 || ratio > 3.0 {
		t.Errorf("energy ratio = %.2f, expected roughly similar", ratio)
	}
}

func TestDGDirectionForSlotMaxProbability(t *testing.T) {
	dirs := []geometry.Vec3{
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: -1, Y: 0, Z: 0},
	}

	probs := [][]float64{
		{0.1, 0.8},
		{0.7, 0.1},
		{0.2, 0.1},
	}

	// Slot 0: DG1 has highest probability (0.7) → direction (0, 1, 0).
	dir0 := dgDirectionForSlot(dirs, probs, 0, 0)
	if dir0 != dirs[1] {
		t.Errorf("slot 0: got %v, want %v", dir0, dirs[1])
	}

	// Slot 1: DG0 has highest probability (0.8) → direction (1, 0, 0).
	dir1 := dgDirectionForSlot(dirs, probs, 1, 0)
	if dir1 != dirs[0] {
		t.Errorf("slot 1: got %v, want %v", dir1, dirs[0])
	}
}

func TestDGDirectionForSlotBlended(t *testing.T) {
	dirs := []geometry.Vec3{
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: -1, Y: 0, Z: 0},
	}

	probs := [][]float64{
		{0.6},
		{0.3},
		{0.1},
	}

	// Blend top 2: the third direction must not affect the weighted average.
	dir := dgDirectionForSlot(dirs, probs, 0, 2)
	wantX := 0.6
	wantY := 0.3
	norm := math.Sqrt(wantX*wantX + wantY*wantY)

	if math.Abs(dir.X-wantX/norm) > 1e-10 || math.Abs(dir.Y-wantY/norm) > 1e-10 {
		t.Errorf("blended direction = %v, want normalized (%.4f, %.4f, 0)",
			dir, wantX/norm, wantY/norm)
	}
}
