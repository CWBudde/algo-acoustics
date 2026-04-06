package hybrid

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/raytrace"
)

// TestDGBinauralListeningFixture exports both random-direction and DG-weighted
// binaural IRs as stereo WAV files for subjective listening comparison.
//
// Run with: go test -v -run TestDGBinauralListeningFixture ./hybrid/
//
// Output files are written to testdata/regression/brir_*.wav.
func TestDGBinauralListeningFixture(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join("..", "testdata", "regression")
	_, statErr := os.Stat(outDir)

	if os.IsNotExist(statErr) {
		t.Skipf("regression directory does not exist: %s", outDir)
	}

	spec := acoustics.Octave6
	sampleRate := 44100
	volume := 200.0
	binDuration := 0.005
	duration := 1.0

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
	itdHRTF := itdDataset{sampleRate: sampleRate, itdSamples: 10}

	// --- Random-direction variant ---
	randomCfg := ir.BinauralPoissonConfig{
		Bins:        bins,
		BinDuration: binDuration,
		Volume:      volume,
		BandSpec:    spec,
		SampleRate:  sampleRate,
		HRTF:        itdHRTF,
	}

	randomLeft, randomRight, err := ir.RenderBinauralPoisson(randomCfg, rand.New(rand.NewSource(42)))
	if err != nil {
		t.Fatalf("random BRIR render error: %v", err)
	}

	randomPath := filepath.Join(outDir, "brir_random.wav")

	err = export.WriteStereoWAV(randomPath, randomLeft, randomRight)
	if err != nil {
		t.Fatalf("write random WAV error: %v", err)
	}

	t.Logf("wrote random BRIR: %s (%d samples)", randomPath, randomLeft.Len())

	// --- DG-weighted variant (consistent left direction) ---
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

	for k := range bins {
		dgProbs[1][k] = 1.0 // all from left
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

	dgLeft, dgRight, err := ir.RenderBinauralPoisson(dgCfg, rand.New(rand.NewSource(42)))
	if err != nil {
		t.Fatalf("DG BRIR render error: %v", err)
	}

	dgPath := filepath.Join(outDir, "brir_dg_left.wav")

	err = export.WriteStereoWAV(dgPath, dgLeft, dgRight)
	if err != nil {
		t.Fatalf("write DG WAV error: %v", err)
	}

	t.Logf("wrote DG BRIR: %s (%d samples)", dgPath, dgLeft.Len())
}
