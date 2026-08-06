package ism

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// TestISMBandGainsArePhysicallyBounded verifies that for a scene using valid
// absorption coefficients (0 ≤ α ≤ 1 per band), all returned BandGain values
// stay within [-1, 1]. Exceeding this range would indicate a physically
// impossible reflection coefficient.
func TestISMBandGainsArePhysicallyBounded(t *testing.T) {
	t.Parallel()

	sc := scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width: 6, Depth: 5, Height: 3,
				WallMaterials: [6]string{"m", "m", "m", "m", "m", "m"},
			},
		},
		Materials: map[string]scene.Material{
			"m": {
				Name:             "m",
				AbsorptionByBand: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6},
				ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Directivity: directivity.OmniModel{},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 4, Y: 3, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	solver := ISMSolver{}

	events, err := solver.Solve(&sc, ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	for i, event := range events {
		for b, gain := range event.BandGain {
			if gain < -1-1e-9 || gain > 1+1e-9 {
				t.Fatalf("event %d (kind=%v) band %d: BandGain = %v, want in [-1, 1]", i, event.Kind, b, gain)
			}
		}
	}
}

// TestISMNormalIncidenceBandGainMatchesPressureReflectance verifies that for a
// geometry where the first-order reflection from the x=0 wall is at normal
// incidence, the band gain equals sqrt(1−α) (the pressure reflectance).
// The testScene has source at (1,2,3) and receiver at (4,2,3), so the image
// source is at (−1,2,3), and the path to the receiver is purely along x → normal.
func TestISMNormalIncidenceBandGainMatchesPressureReflectance(t *testing.T) {
	t.Parallel()

	alphas := []float64{0.0, 0.25, 0.5, 0.75}
	for _, alpha := range alphas {
		sc := testScene(t)
		// Make all walls absorptive so only the x=0 first-order specular survives.
		sc.Room.Shoebox.WallMaterials = [6]string{"hard", "soft", "soft", "soft", "soft", "soft"}
		sc.Materials["soft"] = scene.Material{
			Name:             "soft",
			AbsorptionByBand: []float64{1, 1, 1, 1, 1, 1},
			ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
		}
		sc.Materials["hard"] = scene.Material{
			Name:             "hard",
			AbsorptionByBand: []float64{alpha, alpha, alpha, alpha, alpha, alpha},
			ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
		}

		solver := ISMSolver{}

		events, err := solver.Solve(&sc, ISMConfig{
			MaxOrder:     1,
			SpeedOfSound: acoustics.SpeedOfSound,
			BandSpec:     sc.BandSpec,
		})
		if err != nil {
			t.Fatalf("Solve(α=%v) error = %v", alpha, err)
		}

		// Find the specular event from the x=0 wall (direction: +x from receiver side).
		var found *ir.Event

		for i := range events {
			if events[i].Kind == ir.EventSpecular && directionMatches(events[i].Direction, geometry.Vec3{X: -1}) {
				found = &events[i]
				break
			}
		}

		if found == nil {
			t.Fatalf("α=%v: no x=0 wall reflection found", alpha)
		}

		wantGain := math.Sqrt(1 - alpha)
		for b, gain := range found.BandGain {
			if math.Abs(gain-wantGain) > 1e-9 {
				t.Fatalf("α=%v band %d: BandGain = %v, want sqrt(1−α) = %v", alpha, b, gain, wantGain)
			}
		}
	}
}

// TestISMHighAbsorptionSpecularContributionIsNegligible verifies that with
// near-total absorption (α=0.99), the rendered contribution of each specular
// event (amplitude × avgBandGain) is much smaller than the direct path.
// sqrt(1−0.99) = 0.1 per bounce, so a first-order specular contribution is
// at most 10 % of a same-distance direct event.
func TestISMHighAbsorptionSpecularContributionIsNegligible(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	for name, mat := range sc.Materials {
		mat.AbsorptionByBand = []float64{0.99, 0.99, 0.99, 0.99, 0.99, 0.99}
		sc.Materials[name] = mat
	}

	solver := ISMSolver{}

	events, err := solver.Solve(&sc, ISMConfig{
		MaxOrder:     1,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	directEvent := firstDirectEvent(events)
	if directEvent == nil {
		t.Fatal("expected a direct event")
	}

	directContrib := directEvent.Amplitude // BandGain empty → avg = 1

	for i, event := range events {
		if event.Kind != ir.EventSpecular {
			continue
		}

		avgGain := avgBandGain(event.BandGain)
		specularContrib := event.Amplitude * avgGain
		// The rendered specular contribution must be < 15 % of the direct.
		// (Geometric factor: reflection is farther than direct; band gain ≈ 0.1.)
		if specularContrib > 0.15*directContrib {
			t.Fatalf("event %d: specular contrib = %v, direct = %v (ratio %.3f > 0.15) for α=0.99",
				i, specularContrib, directContrib, specularContrib/directContrib)
		}
	}
}

func avgBandGain(gains []float64) float64 {
	if len(gains) == 0 {
		return 1
	}

	sum := 0.0
	for _, g := range gains {
		sum += g
	}

	return sum / float64(len(gains))
}

// TestISMAmplitudesArePositive verifies that all event amplitudes are strictly
// positive. Amplitude encodes magnitude; sign is carried by phase and band
// gains. A zero or negative amplitude would indicate a bug in 1/r scaling.
func TestISMAmplitudesArePositive(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}

	events, err := solver.Solve(func() *scene.Scene { s := testScene(t); return &s }(), ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	for i, event := range events {
		if event.Amplitude <= 0 {
			t.Fatalf("event %d (kind=%v, t=%.6f): Amplitude = %v, want > 0", i, event.Kind, event.TimeSeconds, event.Amplitude)
		}

		if math.IsNaN(event.Amplitude) || math.IsInf(event.Amplitude, 0) {
			t.Fatalf("event %d: Amplitude = %v, want finite positive", i, event.Amplitude)
		}
	}
}
