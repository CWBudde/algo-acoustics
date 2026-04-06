package hybrid

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/raytrace"
)

// TestPoissonListeningFixture exports legacy and Poisson late-field RIRs as
// WAV files for subjective A/B comparison. The test is skipped by default;
// run with -run TestPoissonListeningFixture to generate the files.
func TestPoissonListeningFixture(t *testing.T) {
	if os.Getenv("GENERATE_FIXTURES") == "" {
		t.Skip("set GENERATE_FIXTURES=1 to generate WAV listening fixtures")
	}

	spec := acoustics.Octave8
	sampleRate := 44100
	volume := 200.0
	binDuration := 0.005
	duration := 1.0

	// Build a realistic energy histogram with exponential decay and
	// frequency-dependent absorption (higher bands decay faster).
	binCount := int(math.Ceil(duration / binDuration))
	hist := raytrace.NewEnergyHistogram(duration, binDuration, spec.BandCount())

	// Per-band T60 values (typical room with moderate absorption).
	bandT60 := []float64{1.2, 1.1, 1.0, 0.9, 0.8, 0.7, 0.6, 0.5}

	for i := range binCount {
		tBin := (float64(i) + 0.5) * binDuration
		energy := make([]float64, spec.BandCount())

		for b := range energy {
			decayRate := -6.0 * math.Log(10) / bandT60[b]
			energy[b] = 0.01 * math.Exp(decayRate*tBin)
		}

		hist.Add(tBin, energy)
	}

	outDir := filepath.Join("testdata", "listening")

	err := os.MkdirAll(outDir, 0o755)
	if err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	// Legacy RIR.
	legacy := HistogramToBuffer(hist, sampleRate)
	normalizePeak(legacy)

	err = export.WriteMonoWAV(filepath.Join(outDir, "late_legacy.wav"), legacy)
	if err != nil {
		t.Fatalf("write legacy WAV: %v", err)
	}

	t.Logf("wrote %s (%d samples, %.2f s)", filepath.Join(outDir, "late_legacy.wav"), legacy.Len(), legacy.Duration())

	// Poisson RIR.
	rng := rand.New(rand.NewSource(42))

	poisson, err := HistogramToPoissonBuffer(hist, volume, spec, sampleRate, rng)
	if err != nil {
		t.Fatalf("HistogramToPoissonBuffer() error: %v", err)
	}

	normalizePeak(poisson)

	err = export.WriteMonoWAV(filepath.Join(outDir, "late_poisson.wav"), poisson)
	if err != nil {
		t.Fatalf("write poisson WAV: %v", err)
	}

	t.Logf("wrote %s (%d samples, %.2f s)", filepath.Join(outDir, "late_poisson.wav"), poisson.Len(), poisson.Duration())
}

// normalizePeak scales the buffer so the peak amplitude is 0.9.
func normalizePeak(buf *ir.Buffer) {
	if buf == nil || len(buf.Samples) == 0 {
		return
	}

	var peak float64
	for _, s := range buf.Samples {
		if a := math.Abs(s); a > peak {
			peak = a
		}
	}

	if peak <= 0 {
		return
	}

	scale := 0.9 / peak
	for i := range buf.Samples {
		buf.Samples[i] *= scale
	}
}
