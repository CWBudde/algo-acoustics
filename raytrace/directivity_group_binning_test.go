package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// newTestSceneForDG creates a shoebox scene with a fully reflective room
// and centered source/receiver suitable for DG binning tests.
func newTestSceneForDG() *scene.Scene {
	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  6,
				Height: 6,
				WallMaterials: [6]string{
					"reflective", "reflective", "reflective",
					"reflective", "reflective", "reflective",
				},
			},
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 3, Y: 3, Z: 3}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3, Y: 3, Z: 3}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func TestTraceWithDGsPopulatesHistograms(t *testing.T) {
	t.Parallel()

	sc := newTestSceneForDG()
	dgs := NewDirectivityGroups(4, 2)

	tracer := RayTracer{
		Config: LaunchConfig{
			NumRays:        2000,
			MaxBounces:     5,
			MaxTimeSeconds: 0.5,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:             sc,
		DirectivityGroups: dgs,
	}

	_, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	// After tracing, each DG should have a non-nil histogram.
	for i, dg := range tracer.DirectivityGroups {
		if dg.Histogram == nil {
			t.Errorf("DG[%d] histogram is nil after Trace()", i)
		}
	}
}

func TestTraceWithDGsIsotropicDistribution(t *testing.T) {
	t.Parallel()

	// Symmetric cube with source at center — all DGs should receive
	// approximately equal energy due to the isotropic field.
	sc := newTestSceneForDG()
	dgs := NewDirectivityGroups(4, 2)

	tracer := RayTracer{
		Config: LaunchConfig{
			NumRays:        50000,
			MaxBounces:     10,
			MaxTimeSeconds: 0.5,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:             sc,
		DirectivityGroups: dgs,
	}

	_, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	// Sum total energy per DG across all bins and bands.
	dgEnergies := make([]float64, len(tracer.DirectivityGroups))
	var totalEnergy float64

	for i, dg := range tracer.DirectivityGroups {
		if dg.Histogram == nil {
			continue
		}

		for _, bin := range dg.Histogram.Bins {
			for _, e := range bin.BandEnergy {
				dgEnergies[i] += e
				totalEnergy += e
			}
		}
	}

	if totalEnergy <= 0 {
		t.Fatal("no energy accumulated in any DG")
	}

	// Each DG should have roughly 1/N of total energy.
	// Allow 30% tolerance due to Monte Carlo variance.
	expected := totalEnergy / float64(len(dgs))
	tolerance := 0.30

	for i, e := range dgEnergies {
		ratio := e / expected
		if math.Abs(ratio-1) > tolerance {
			t.Errorf("DG[%d] energy ratio = %.3f (got %.4f, expected ~%.4f), exceeds %.0f%% tolerance",
				i, ratio, e, expected, tolerance*100)
		}
	}
}

func TestTraceWithDGsSingleDirectionHit(t *testing.T) {
	t.Parallel()

	// Place source on the +X side of the receiver. Early reflections from the
	// +X wall arrive from the -X direction (back toward the source), so the
	// highest-energy DG should be roughly azimuth ~180 deg.
	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  10,
				Depth:  10,
				Height: 10,
				WallMaterials: [6]string{
					"reflective", "reflective", "reflective",
					"reflective", "reflective", "reflective",
				},
			},
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 9, Y: 5, Z: 5}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 5, Y: 5, Z: 5}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	dgs := NewDirectivityGroups(4, 2)

	tracer := RayTracer{
		Config: LaunchConfig{
			NumRays:        10000,
			MaxBounces:     1,
			MaxTimeSeconds: 0.5,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:             sc,
		ReceiverRadius:    0.5,
		DirectivityGroups: dgs,
	}

	_, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	// Find the DG with the highest energy — it should be in the +X direction
	// (rays coming from source at +X pass through receiver heading in +X).
	var maxEnergy float64
	maxIdx := -1

	for i, dg := range tracer.DirectivityGroups {
		if dg.Histogram == nil {
			continue
		}

		var e float64

		for _, bin := range dg.Histogram.Bins {
			for _, be := range bin.BandEnergy {
				e += be
			}
		}

		if e > maxEnergy {
			maxEnergy = e
			maxIdx = i
		}
	}

	if maxIdx < 0 {
		t.Fatal("no energy in any DG")
	}

	// The dominant DG should have its azimuth center near 0 deg (front/+X)
	// since direct rays from +X source pass through the receiver in the +X direction.
	dg := tracer.DirectivityGroups[maxIdx]
	azDeg := dg.AzimuthCenter * 180 / math.Pi

	// Allow the dominant to be in the front half (azimuth < 90 or > 270)
	if azDeg > 90 && azDeg < 270 {
		t.Errorf("dominant DG azimuth = %.1f deg, expected near 0 deg (+X direction)", azDeg)
	}
}

func TestTraceWithDGsDiffuseRainBinning(t *testing.T) {
	t.Parallel()

	// With diffuse rain enabled, rain contributions should also be binned into DGs.
	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  6,
				Height: 6,
				WallMaterials: [6]string{
					"scattering", "scattering", "scattering",
					"scattering", "scattering", "scattering",
				},
			},
		},
		Materials: map[string]scene.Material{
			"scattering": {
				AbsorptionByBand: []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
				ScatteringByBand: []float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9},
			},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 3, Y: 3, Z: 3}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3, Y: 3, Z: 3}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	dgs := NewDirectivityGroups(4, 2)

	tracer := RayTracer{
		Config: LaunchConfig{
			NumRays:        10000,
			MaxBounces:     5,
			MaxTimeSeconds: 0.5,
			SpeedOfSound:   acoustics.SpeedOfSound,
			DiffuseRain:    true,
		},
		Scene:             sc,
		DirectivityGroups: dgs,
	}

	_, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	// With high scattering and centered source, DG energy should be non-zero
	// and roughly isotropic.
	var totalDGEnergy float64
	nonZeroCount := 0

	for _, dg := range tracer.DirectivityGroups {
		if dg.Histogram == nil {
			continue
		}

		var e float64

		for _, bin := range dg.Histogram.Bins {
			for _, be := range bin.BandEnergy {
				e += be
			}
		}

		if e > 0 {
			nonZeroCount++
		}

		totalDGEnergy += e
	}

	if totalDGEnergy <= 0 {
		t.Fatal("no DG energy with diffuse rain enabled")
	}

	// All DGs should have received some energy in a symmetric isotropic field.
	if nonZeroCount < len(dgs) {
		t.Errorf("only %d/%d DGs received energy, expected all", nonZeroCount, len(dgs))
	}
}

func TestTraceWithoutDGsIsUnchanged(t *testing.T) {
	t.Parallel()

	// When DirectivityGroups is nil, Trace() should behave exactly as before.
	sc := newTestSceneForDG()

	tracer := RayTracer{
		Config: LaunchConfig{
			NumRays:        1000,
			MaxBounces:     3,
			MaxTimeSeconds: 0.3,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene: sc,
	}

	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	if len(hist.Bins) == 0 {
		t.Fatal("expected non-empty histogram")
	}

	// DirectivityGroups should remain nil.
	if tracer.DirectivityGroups != nil {
		t.Error("DirectivityGroups should remain nil when not configured")
	}
}

func TestTraceWithDGsMainHistogramUnchanged(t *testing.T) {
	t.Parallel()

	// The main histogram should contain the same energy regardless of DG binning.
	sc := newTestSceneForDG()
	cfg := LaunchConfig{
		NumRays:        5000,
		MaxBounces:     5,
		MaxTimeSeconds: 0.3,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}

	// Trace without DGs.
	tracerNoDG := RayTracer{Config: cfg, Scene: sc}

	histNoDG, err := tracerNoDG.Trace()
	if err != nil {
		t.Fatalf("Trace() without DGs error = %v", err)
	}

	// Trace with DGs.
	dgs := NewDirectivityGroups(4, 2)
	tracerDG := RayTracer{Config: cfg, Scene: sc, DirectivityGroups: dgs}

	histDG, err := tracerDG.Trace()
	if err != nil {
		t.Fatalf("Trace() with DGs error = %v", err)
	}

	// Total energy in main histogram should be identical (same RNG seed).
	energyNoDG := sumAllHistEnergy(histNoDG)
	energyDG := sumAllHistEnergy(histDG)

	if math.Abs(energyNoDG-energyDG) > 1e-10 {
		t.Errorf("main histogram energy changed with DGs: without=%.6f, with=%.6f",
			energyNoDG, energyDG)
	}
}

func sumAllHistEnergy(h *EnergyHistogram) float64 {
	var total float64

	for _, bin := range h.Bins {
		for _, e := range bin.BandEnergy {
			total += e
		}
	}

	return total
}
