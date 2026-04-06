package hybrid

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/raytrace"
)

// TestDGBinauralVsRandomAB compares the DG-weighted binaural Poisson synthesis
// against the original random-direction variant. Both use the same energy
// histogram but different HRIR selection strategies.
//
// Requirements:
//   - T30 should agree within 3% (same energy decay regardless of direction).
//   - IACC should differ (DG version has coherent directionality; random does not).
func TestDGBinauralVsRandomAB(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	sampleRate := 44100
	volume := 200.0
	binDuration := 0.005
	duration := 0.5

	binCount := int(math.Ceil(duration / binDuration))
	hist := raytrace.NewEnergyHistogram(duration, binDuration, spec.BandCount())

	t60 := 0.8
	decayRate := -6.0 * math.Log(10) / t60
	bandRatios := []float64{1, 2, 4, 4, 2, 1}

	for i := range binCount {
		tBin := (float64(i) + 0.5) * binDuration
		envelope := math.Exp(decayRate * tBin)

		energy := make([]float64, spec.BandCount())
		for b := range energy {
			energy[b] = 0.01 * bandRatios[b] * envelope
		}

		hist.Add(tBin, energy)
	}

	bins := histogramToIRBins(hist)

	// Build a simple ITD-based HRTF that produces measurable L/R differences.
	itdHRTF := itdDataset{sampleRate: sampleRate, itdSamples: 10}

	// --- Random-direction variant (baseline) ---
	randomCfg := ir.BinauralPoissonConfig{
		Bins:        bins,
		BinDuration: binDuration,
		Volume:      volume,
		BandSpec:    spec,
		SampleRate:  sampleRate,
		HRTF:        itdHRTF,
	}

	rngRandom := rand.New(rand.NewSource(42))

	randomLeft, randomRight, err := ir.RenderBinauralPoisson(randomCfg, rngRandom)
	if err != nil {
		t.Fatalf("random BRIR render error: %v", err)
	}

	// --- DG-weighted variant ---
	// Set up 4 DGs (front, left, back, right) with all energy from the left.
	dgDirs := []geometry.Vec3{
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: -1, Y: 0, Z: 0},
		{X: 0, Y: -1, Z: 0},
	}

	dgProbs := make([][]float64, 4)
	for d := range dgProbs {
		dgProbs[d] = make([]float64, len(bins))
	}

	// All slots dominated by DG 1 (left / +Y direction).
	for k := range bins {
		dgProbs[1][k] = 1.0
	}

	dgCfg := ir.BinauralPoissonConfig{
		Bins:            bins,
		BinDuration:     binDuration,
		Volume:          volume,
		BandSpec:        spec,
		SampleRate:      sampleRate,
		HRTF:            itdHRTF,
		DGDirections:    dgDirs,
		DGProbabilities: dgProbs,
	}

	rngDG := rand.New(rand.NewSource(42))

	dgLeft, dgRight, err := ir.RenderBinauralPoisson(dgCfg, rngDG)
	if err != nil {
		t.Fatalf("DG BRIR render error: %v", err)
	}

	// Equalize lengths.
	equalizeBufferLengths(randomLeft, randomRight, dgLeft, dgRight)

	// --- T30 comparison ---
	// Use left channel for T30 (mono decay metric).
	randomT30, err := metrics.T30(randomLeft)
	if err != nil {
		t.Fatalf("random T30 error: %v", err)
	}

	dgT30, err := metrics.T30(dgLeft)
	if err != nil {
		t.Fatalf("DG T30 error: %v", err)
	}

	t30RelErr := math.Abs(dgT30-randomT30) / randomT30
	t.Logf("T30: random=%.4f s, DG=%.4f s, relative error=%.2f%%",
		randomT30, dgT30, t30RelErr*100)

	if t30RelErr > 0.03 {
		t.Errorf("T30 relative error %.2f%% exceeds 3%% threshold", t30RelErr*100)
	}

	// --- IACC comparison ---
	randomIACC, err := metrics.IACC(randomLeft, randomRight)
	if err != nil {
		t.Fatalf("random IACC error: %v", err)
	}

	dgIACC, err := metrics.IACC(dgLeft, dgRight)
	if err != nil {
		t.Fatalf("DG IACC error: %v", err)
	}

	t.Logf("IACC: random=%.4f, DG=%.4f", randomIACC, dgIACC)

	// The DG variant with a consistent lateral direction should have different
	// IACC than the random variant. The random variant averages over all
	// directions (IACC closer to 1.0 with NoopHRTF or varying with ITD),
	// while the DG variant consistently uses a lateral direction (producing
	// a consistent ITD → lower IACC due to systematic L/R delay).
	if randomIACC == dgIACC {
		t.Errorf("IACC values are identical (%.4f), expected them to differ", randomIACC)
	}
}

// histogramToIRBins converts a raytrace histogram to ir.EnergyBin slice.
func histogramToIRBins(h *raytrace.EnergyHistogram) []ir.EnergyBin {
	bins := make([]ir.EnergyBin, len(h.Bins))
	for i, hb := range h.Bins {
		bins[i] = ir.EnergyBin{
			TimeSeconds: hb.TimeSeconds,
			BandEnergy:  append([]float64(nil), hb.BandEnergy...),
		}
	}

	return bins
}

// equalizeBufferLengths pads all buffers to the length of the longest one.
func equalizeBufferLengths(bufs ...*ir.Buffer) {
	maxLen := 0
	for _, b := range bufs {
		if b.Len() > maxLen {
			maxLen = b.Len()
		}
	}

	for _, b := range bufs {
		if b.Len() < maxLen {
			padded := make([]float64, maxLen)
			copy(padded, b.Samples)
			b.Samples = padded
		}
	}
}

// itdDataset returns different delays for left vs right based on direction,
// simulating interaural time difference for lateralized sources.
type itdDataset struct {
	sampleRate int
	itdSamples int
}

func (d itdDataset) SampleRate() int { return d.sampleRate }

func (d itdDataset) Lookup(dir geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	if dir.Y > 0 {
		padSamples := d.itdSamples
		rightHRIR := make([]float64, padSamples+1)
		rightHRIR[padSamples] = 1

		return []float64{1}, rightHRIR, 0, nil
	}

	if dir.Y < 0 {
		padSamples := d.itdSamples
		leftHRIR := make([]float64, padSamples+1)
		leftHRIR[padSamples] = 1

		return leftHRIR, []float64{1}, 0, nil
	}

	return []float64{1}, []float64{1}, 0, nil
}
