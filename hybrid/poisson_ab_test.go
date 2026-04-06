package hybrid

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/raytrace"
)

// TestPoissonVsLegacyAB compares the Poisson RIR synthesis against the legacy
// random-noise method. Both should produce statistically equivalent decay
// characteristics (T30 and EDT within 3%). The spectral envelope is verified
// via a self-consistency check on the Poisson output: per-band energy ratios
// must match the input histogram within 1 dB.
func TestPoissonVsLegacyAB(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	sampleRate := 44100
	volume := 200.0 // m³
	binDuration := 0.005
	duration := 0.5

	// Build a synthetic energy histogram with exponential decay and
	// frequency-dependent band energy (higher bands get more energy).
	binCount := int(math.Ceil(duration / binDuration))
	hist := raytrace.NewEnergyHistogram(duration, binDuration, spec.BandCount())

	t60 := 0.8
	decayRate := -6.0 * math.Log(10) / t60

	// Target per-band energy ratios (arbitrary but non-uniform to test shape).
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

	// Render legacy.
	legacy := HistogramToBuffer(hist, sampleRate)

	// Render Poisson.
	rng := rand.New(rand.NewSource(42))

	poisson, err := HistogramToPoissonBuffer(hist, volume, spec, sampleRate, rng)
	if err != nil {
		t.Fatalf("HistogramToPoissonBuffer() error: %v", err)
	}

	// Equalize lengths for comparison.
	maxLen := max(legacy.Len(), poisson.Len())
	if legacy.Len() < maxLen {
		padded := make([]float64, maxLen)
		copy(padded, legacy.Samples)
		legacy.Samples = padded
	}

	if poisson.Len() < maxLen {
		padded := make([]float64, maxLen)
		copy(padded, poisson.Samples)
		poisson.Samples = padded
	}

	// Compare T30.
	legacyT30, err := metrics.T30(legacy)
	if err != nil {
		t.Fatalf("legacy T30 error: %v", err)
	}

	poissonT30, err := metrics.T30(poisson)
	if err != nil {
		t.Fatalf("poisson T30 error: %v", err)
	}

	t30RelErr := math.Abs(poissonT30-legacyT30) / legacyT30
	t.Logf("T30: legacy=%.4f s, poisson=%.4f s, relative error=%.2f%%", legacyT30, poissonT30, t30RelErr*100)

	if t30RelErr > 0.03 {
		t.Errorf("T30 relative error %.2f%% exceeds 3%% threshold", t30RelErr*100)
	}

	// Compare EDT.
	legacyEDT, err := metrics.EDT(legacy)
	if err != nil {
		t.Fatalf("legacy EDT error: %v", err)
	}

	poissonEDT, err := metrics.EDT(poisson)
	if err != nil {
		t.Fatalf("poisson EDT error: %v", err)
	}

	edtRelErr := math.Abs(poissonEDT-legacyEDT) / legacyEDT
	t.Logf("EDT: legacy=%.4f s, poisson=%.4f s, relative error=%.2f%%", legacyEDT, poissonEDT, edtRelErr*100)

	if edtRelErr > 0.03 {
		t.Errorf("EDT relative error %.2f%% exceeds 3%% threshold", edtRelErr*100)
	}

	// Spectral self-consistency: verify the Poisson output's per-band energy
	// ratios match the input histogram. Compute band levels of the Poisson
	// buffer and check that relative levels match the input bandRatios.
	rows, err := metrics.CompareBuffers(poisson, poisson, spec)
	if err != nil {
		t.Fatalf("CompareBuffers() error: %v", err)
	}

	var bandLevels []float64

	for _, row := range rows {
		if row.Unit == "dB" {
			bandLevels = append(bandLevels, row.Expected)
		}
	}

	if len(bandLevels) != spec.BandCount() {
		t.Fatalf("expected %d band levels, got %d", spec.BandCount(), len(bandLevels))
	}

	// Convert bandRatios to dB relative to band 0.
	refLevel := bandLevels[0]
	refRatio := bandRatios[0]

	for b := 1; b < spec.BandCount(); b++ {
		expectedDeltaDB := 10 * math.Log10(bandRatios[b]/refRatio)
		actualDeltaDB := bandLevels[b] - refLevel
		shapeDelta := actualDeltaDB - expectedDeltaDB

		t.Logf("band %d (%.0f Hz): expected_delta=%.2f dB, actual_delta=%.2f dB, shape_error=%.2f dB",
			b, spec.CenterFreqs[b], expectedDeltaDB, actualDeltaDB, shapeDelta)

		if math.Abs(shapeDelta) > 1.0 {
			t.Errorf("band %d: spectral shape error %.2f dB exceeds 1 dB threshold", b, shapeDelta)
		}
	}
}
