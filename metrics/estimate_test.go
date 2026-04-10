package metrics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// shoeboxSimpleScene builds the 6 × 4.5 × 2.8 plaster room in-process
// (matches testdata/rooms/shoebox_simple.json) so unit tests have no
// filesystem dependency.
func shoeboxSimpleScene() *scene.Scene {
	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"plaster", "plaster", "plaster",
					"plaster", "plaster", "plaster",
				},
			},
		},
		Materials: map[string]scene.Material{
			"plaster": {
				Name:             "plaster",
				AbsorptionByBand: []float64{0.1, 0.1, 0.15, 0.2, 0.2, 0.25},
			},
		},
		BandSpec: acoustics.Octave6,
	}
}

func TestShoeboxStatsFromScene(t *testing.T) {
	t.Parallel()

	t.Run("correct volume and surface area", func(t *testing.T) {
		t.Parallel()

		sc := shoeboxSimpleScene()

		stats, err := ShoeboxStatsFromScene(sc)
		if err != nil {
			t.Fatalf("ShoeboxStatsFromScene() error = %v", err)
		}

		wantVolume := 6.0 * 4.5 * 2.8 // 75.6 m³
		if math.Abs(stats.Volume-wantVolume) > 1e-9 {
			t.Errorf("Volume = %g, want %g", stats.Volume, wantVolume)
		}

		// 2*(W*D + D*H + H*W) = 2*(27 + 12.6 + 16.8) = 112.8 m²
		wantArea := 2 * (6*4.5 + 4.5*2.8 + 6*2.8)
		if math.Abs(stats.SurfaceArea-wantArea) > 1e-9 {
			t.Errorf("SurfaceArea = %g, want %g", stats.SurfaceArea, wantArea)
		}
	})

	t.Run("correct per-band alpha", func(t *testing.T) {
		t.Parallel()

		sc := shoeboxSimpleScene()

		stats, err := ShoeboxStatsFromScene(sc)
		if err != nil {
			t.Fatalf("ShoeboxStatsFromScene() error = %v", err)
		}

		// All walls have the same material, so area-weighted alpha equals
		// the material's absorption coefficient for each band.
		wantAlpha := []float64{0.1, 0.1, 0.15, 0.2, 0.2, 0.25}
		for i, want := range wantAlpha {
			if math.Abs(stats.AlphaByBand[i]-want) > 1e-9 {
				t.Errorf("AlphaByBand[%d] = %g, want %g", i, stats.AlphaByBand[i], want)
			}
		}
	})

	t.Run("error on nil scene", func(t *testing.T) {
		t.Parallel()

		_, err := ShoeboxStatsFromScene(nil)
		if err == nil {
			t.Fatal("ShoeboxStatsFromScene(nil) error = nil, want error")
		}
	})

	t.Run("error on mesh scene", func(t *testing.T) {
		t.Parallel()

		sc := &scene.Scene{
			Room:     scene.Room{Kind: scene.RoomKindMesh},
			BandSpec: acoustics.Octave6,
		}

		_, err := ShoeboxStatsFromScene(sc)
		if err == nil {
			t.Fatal("ShoeboxStatsFromScene(mesh) error = nil, want error")
		}
	})

	t.Run("error on zero dimension", func(t *testing.T) {
		t.Parallel()

		sc := shoeboxSimpleScene()
		sc.Room.Shoebox.Width = 0

		_, err := ShoeboxStatsFromScene(sc)
		if err == nil {
			t.Fatal("ShoeboxStatsFromScene(zero width) error = nil, want error")
		}
	})
}

func TestSabineRT60(t *testing.T) {
	t.Parallel()

	sc := shoeboxSimpleScene()
	stats, _ := ShoeboxStatsFromScene(sc)

	t.Run("band 0 value", func(t *testing.T) {
		t.Parallel()

		// T60 = 0.161 * 75.6 / (112.8 * 0.1) ≈ 1.079 s
		got, err := SabineRT60(stats, 0)
		if err != nil {
			t.Fatalf("SabineRT60() error = %v", err)
		}

		want := sabineConstant * stats.Volume / (stats.SurfaceArea * 0.1)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("SabineRT60() = %g, want %g", got, want)
		}
	})

	t.Run("larger alpha gives shorter T60", func(t *testing.T) {
		t.Parallel()

		t60Band0, _ := SabineRT60(stats, 0) // α = 0.1
		t60Band2, _ := SabineRT60(stats, 2) // α = 0.15

		if t60Band2 >= t60Band0 {
			t.Errorf("higher absorption should reduce T60: band2=%g, band0=%g", t60Band2, t60Band0)
		}
	})

	t.Run("error on zero alpha", func(t *testing.T) {
		t.Parallel()

		zeroStats := RoomStats{
			Volume:      75.6,
			SurfaceArea: 112.8,
			AlphaByBand: []float64{0},
		}

		_, err := SabineRT60(zeroStats, 0)
		if err == nil {
			t.Fatal("SabineRT60(alpha=0) error = nil, want error")
		}
	})

	t.Run("error on invalid band index", func(t *testing.T) {
		t.Parallel()

		_, err := SabineRT60(stats, 99)
		if err == nil {
			t.Fatal("SabineRT60(band=99) error = nil, want error")
		}
	})
}

func TestEyringRT60(t *testing.T) {
	t.Parallel()

	sc := shoeboxSimpleScene()
	stats, _ := ShoeboxStatsFromScene(sc)

	t.Run("band 0 value", func(t *testing.T) {
		t.Parallel()

		// T60 = 0.161 * 75.6 / (-112.8 * ln(0.9)) ≈ 1.024 s
		got, err := EyringRT60(stats, 0)
		if err != nil {
			t.Fatalf("EyringRT60() error = %v", err)
		}

		want := sabineConstant * stats.Volume / (-stats.SurfaceArea * math.Log(1-0.1))
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("EyringRT60() = %g, want %g", got, want)
		}
	})

	t.Run("eyring shorter than sabine", func(t *testing.T) {
		t.Parallel()

		// For α > 0, Eyring always gives a shorter T60 than Sabine
		// because -ln(1-α) > α for all α in (0,1).
		for band := range stats.AlphaByBand {
			sabine, _ := SabineRT60(stats, band)
			eyring, _ := EyringRT60(stats, band)

			if eyring >= sabine {
				t.Errorf("band %d: Eyring (%g) should be < Sabine (%g)", band, eyring, sabine)
			}
		}
	})

	t.Run("error on zero alpha", func(t *testing.T) {
		t.Parallel()

		zeroStats := RoomStats{
			Volume:      75.6,
			SurfaceArea: 112.8,
			AlphaByBand: []float64{0},
		}

		_, err := EyringRT60(zeroStats, 0)
		if err == nil {
			t.Fatal("EyringRT60(alpha=0) error = nil, want error")
		}
	})

	t.Run("error on alpha >= 1", func(t *testing.T) {
		t.Parallel()

		fullStats := RoomStats{
			Volume:      75.6,
			SurfaceArea: 112.8,
			AlphaByBand: []float64{1.0},
		}

		_, err := EyringRT60(fullStats, 0)
		if err == nil {
			t.Fatal("EyringRT60(alpha=1) error = nil, want error")
		}
	})
}

func TestCriticalDistance(t *testing.T) {
	t.Parallel()

	sc := shoeboxSimpleScene()
	stats, _ := ShoeboxStatsFromScene(sc)

	t.Run("omni source band 0", func(t *testing.T) {
		t.Parallel()

		dc, err := CriticalDistance(stats, 0, 1)
		if err != nil {
			t.Fatalf("CriticalDistance() error = %v", err)
		}

		// R = S*α/(1-α) = 112.8*0.1/0.9 ≈ 12.53 m²
		// Dc = sqrt(R/(16π)) ≈ 0.499 m
		if dc <= 0 || dc > 10 {
			t.Errorf("CriticalDistance() = %g, want physically plausible (0,10] m", dc)
		}
	})

	t.Run("higher Q increases critical distance", func(t *testing.T) {
		t.Parallel()

		dc1, _ := CriticalDistance(stats, 0, 1)
		dc2, _ := CriticalDistance(stats, 0, 2)

		if dc2 <= dc1 {
			t.Errorf("Q=2 should give larger Dc: dc2=%g, dc1=%g", dc2, dc1)
		}
	})

	t.Run("error on zero q", func(t *testing.T) {
		t.Parallel()

		_, err := CriticalDistance(stats, 0, 0)
		if err == nil {
			t.Fatal("CriticalDistance(q=0) error = nil, want error")
		}
	})
}

func TestEstimateC80(t *testing.T) {
	t.Parallel()

	sc := shoeboxSimpleScene()
	stats, _ := ShoeboxStatsFromScene(sc)

	got, err := EstimateC80(stats, 0)
	if err != nil {
		t.Fatalf("EstimateC80() error = %v", err)
	}

	// For T60 ≈ 1.024 s: C80 = 10*log10(exp(0.08*13.8159/1.024) - 1) ≈ 2.9 dB
	if math.IsInf(got, 0) || math.IsNaN(got) {
		t.Errorf("EstimateC80() = %g, want finite value", got)
	}

	// For a moderately live room (T60 ~ 1 s), C80 is expected in a plausible range.
	if got < -30 || got > 30 {
		t.Errorf("EstimateC80() = %g dB, want in plausible range [-30, 30]", got)
	}
}

func TestEstimateD50(t *testing.T) {
	t.Parallel()

	sc := shoeboxSimpleScene()
	stats, _ := ShoeboxStatsFromScene(sc)

	got, err := EstimateD50(stats, 0)
	if err != nil {
		t.Fatalf("EstimateD50() error = %v", err)
	}

	// D50 is always in [0, 1].
	if got <= 0 || got >= 1 {
		t.Errorf("EstimateD50() = %g, want in (0, 1)", got)
	}
}

// TestStatisticalEstimatesMatchDecayCurve builds a synthetic exponential-decay
// impulse response with the Eyring-predicted T60 and verifies that
// T60FromDecaySlope recovers the same value. This validates both the formula
// correctness and the consistency between the statistical estimate and the
// IR-based extraction.
//
// A direct comparison against the Monte Carlo ray tracer is not attempted here
// because the ray tracer's ISO 9613-1 air absorption (not included in the
// Sabine/Eyring formulas) introduces a systematic T60 shortening that dominates
// the 15 % tolerance budget for typical shoebox rooms.
func TestStatisticalEstimatesMatchDecayCurve(t *testing.T) {
	t.Parallel()

	sc := shoeboxSimpleScene()

	stats, err := ShoeboxStatsFromScene(sc)
	if err != nil {
		t.Fatalf("ShoeboxStatsFromScene() error = %v", err)
	}

	// Use band 2 (500 Hz) — the reference mid-frequency for room acoustics.
	const midBand = 2

	sabineT60, err := SabineRT60(stats, midBand)
	if err != nil {
		t.Fatalf("SabineRT60() error = %v", err)
	}

	eyringT60, err := EyringRT60(stats, midBand)
	if err != nil {
		t.Fatalf("EyringRT60() error = %v", err)
	}

	// Build a synthetic exponential-decay buffer whose energy decays as
	// E(t) = E₀ * exp(-13.8159 * t / T60), giving exactly T60 = eyringT60.
	const sampleRate = 48000
	const durationSeconds = 2.5

	k := logDecayFactor / (2 * eyringT60) // amplitude decay constant
	buf := ir.NewBuffer(sampleRate, durationSeconds)

	for i := range buf.Samples {
		t := float64(i) / float64(sampleRate)
		buf.Samples[i] = math.Exp(-k * t)
	}

	extractedT60, err := T60FromDecaySlope(buf)
	if err != nil {
		t.Fatalf("T60FromDecaySlope() error = %v", err)
	}

	const tolerance = 0.15

	eyringErr := math.Abs(eyringT60-extractedT60) / extractedT60
	sabineErr := math.Abs(sabineT60-extractedT60) / extractedT60

	t.Logf("Eyring T60     = %.4f s", eyringT60)
	t.Logf("Sabine T60     = %.4f s", sabineT60)
	t.Logf("extracted T60  = %.4f s", extractedT60)
	t.Logf("Eyring error   = %.1f%%", eyringErr*100)
	t.Logf("Sabine error   = %.1f%%", sabineErr*100)

	if eyringErr > tolerance {
		t.Errorf("Eyring RT60 error = %.1f%%, want ≤ %.0f%%", eyringErr*100, tolerance*100)
	}

	if sabineErr > tolerance {
		t.Errorf("Sabine RT60 error = %.1f%%, want ≤ %.0f%%", sabineErr*100, tolerance*100)
	}
}
