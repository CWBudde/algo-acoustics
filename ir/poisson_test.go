package ir

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func TestPoissonSequenceDensityMatchesMu(t *testing.T) {
	// Average over many realizations and check that event density at a given
	// time matches the theoretical mu(t) within 5%.
	volume := 2000.0 // m³ — large enough that mu stays below cap at measurement time
	sampleRate := 44100
	duration := 0.5

	c := acoustics.SpeedOfSound
	fourPiC3 := 4 * math.Pi * c * c * c

	// Pick a measurement window where mu is moderate (well below 10,000 cap).
	tCenter := 0.05 // 50 ms
	windowHalf := 0.005
	tLow := tCenter - windowHalf
	tHigh := tCenter + windowHalf

	muTheoretical := fourPiC3 * tCenter * tCenter / volume

	const realizations = 100
	totalEvents := 0

	for i := range realizations {
		rng := rand.New(rand.NewSource(int64(i)))
		seq := PoissonSequence(volume, sampleRate, duration, rng)

		idxLow := int(math.Floor(tLow * float64(sampleRate)))
		idxHigh := int(math.Floor(tHigh * float64(sampleRate)))

		idxHigh = min(idxHigh, len(seq))

		for j := idxLow; j < idxHigh; j++ {
			if seq[j] != 0 {
				totalEvents++
			}
		}
	}

	windowDuration := tHigh - tLow
	avgEvents := float64(totalEvents) / float64(realizations)
	measuredMu := avgEvents / windowDuration

	relErr := math.Abs(measuredMu-muTheoretical) / muTheoretical
	if relErr > 0.05 {
		t.Fatalf("event density at t=%.3f s: measured mu=%.1f, theoretical mu=%.1f, relative error=%.3f (>5%%)",
			tCenter, measuredMu, muTheoretical, relErr)
	}
}

func TestPoissonSequenceT0ForLargeRoom(t *testing.T) {
	// For a 1344 m³ room, t0 ≈ 15.4 ms — no events before this time.
	volume := 1344.0
	sampleRate := 48000
	duration := 0.2

	c := acoustics.SpeedOfSound
	fourPiC3 := 4 * math.Pi * c * c * c
	t0 := math.Cbrt(2 * volume * math.Ln2 / fourPiC3)

	// Verify theoretical t0 is near 15.4 ms.
	if math.Abs(t0-0.0154) > 0.001 {
		t.Fatalf("theoretical t0 = %.4f s, expected ~0.0154 s", t0)
	}

	// Check multiple realizations: no events before t0.
	for i := range 50 {
		rng := rand.New(rand.NewSource(int64(i)))
		seq := PoissonSequence(volume, sampleRate, duration, rng)

		t0Sample := int(math.Floor(t0 * float64(sampleRate)))
		for j := 0; j < t0Sample && j < len(seq); j++ {
			if seq[j] != 0 {
				t.Fatalf("realization %d: event at sample %d (t=%.4f s) before t0=%.4f s",
					i, j, float64(j)/float64(sampleRate), t0)
			}
		}
	}
}

func TestPoissonSequenceMuCap(t *testing.T) {
	// Use a tiny volume so mu exceeds 10,000 quickly, then verify the event
	// rate does not exceed the cap.
	volume := 1.0 // 1 m³ → mu hits cap very early
	sampleRate := 96000
	duration := 0.5

	const realizations = 100
	// Measure density in a window where uncapped mu would be far above 10,000.
	tCenter := 0.3
	windowHalf := 0.05
	tLow := tCenter - windowHalf
	tHigh := tCenter + windowHalf

	totalEvents := 0

	for i := range realizations {
		rng := rand.New(rand.NewSource(int64(i)))
		seq := PoissonSequence(volume, sampleRate, duration, rng)

		idxLow := int(math.Floor(tLow * float64(sampleRate)))
		idxHigh := int(math.Floor(tHigh * float64(sampleRate)))

		idxHigh = min(idxHigh, len(seq))

		for j := idxLow; j < idxHigh; j++ {
			if seq[j] != 0 {
				totalEvents++
			}
		}
	}

	windowDuration := tHigh - tLow
	avgEvents := float64(totalEvents) / float64(realizations)
	measuredMu := avgEvents / windowDuration

	// The measured rate should be at or below the cap (with some tolerance for
	// the "at most one per sample" restriction which reduces effective rate).
	if measuredMu > maxMu*1.05 {
		t.Fatalf("measured mu=%.1f exceeds cap of %.0f (with 5%% tolerance)", measuredMu, maxMu)
	}
}

func TestPoissonSequenceInvalidInputs(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	tests := []struct {
		name       string
		volume     float64
		sampleRate int
		duration   float64
	}{
		{"zero volume", 0, 44100, 1.0},
		{"negative volume", -10, 44100, 1.0},
		{"zero sample rate", 100, 0, 1.0},
		{"zero duration", 100, 44100, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PoissonSequence(tt.volume, tt.sampleRate, tt.duration, rng)
			if result != nil {
				t.Fatalf("expected nil for invalid input, got %d samples", len(result))
			}
		})
	}
}

func TestPoissonSequenceOnlyUnitDeltas(t *testing.T) {
	// Every non-zero sample must be exactly +1 or -1.
	rng := rand.New(rand.NewSource(42))
	seq := PoissonSequence(100, 44100, 0.3, rng)

	for i, v := range seq {
		if v != 0 && v != 1 && v != -1 {
			t.Fatalf("sample %d = %f, want 0, +1, or -1", i, v)
		}
	}
}
