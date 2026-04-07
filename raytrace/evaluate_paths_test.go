package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func newEvaluatePathsRayTracer(absorptionCoeff float64) *RayTracer {
	abs := make([]float64, scene.NumBands)
	for i := range abs {
		abs[i] = absorptionCoeff
	}

	mat := scene.Material{AbsorptionByBand: abs}

	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  10,
				Depth:  8,
				Height: 3,
				WallMaterials: [6]string{
					"walls", "walls", "walls",
					"walls", "floor", "ceiling",
				},
			},
		},
		Materials: map[string]scene.Material{
			"walls":   mat,
			"floor":   mat,
			"ceiling": mat,
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	return &RayTracer{
		Config: LaunchConfig{
			NumRays:        100,
			MaxBounces:     10,
			MaxTimeSeconds: 1,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}
}

func TestEvaluatePaths_ProducesNonEmptyHistogram(t *testing.T) {
	t.Parallel()

	rt := newEvaluatePathsRayTracer(0.1)

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	hist, err := rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths() error = %v", err)
	}

	var totalEnergy float64
	for _, bin := range hist.Bins {
		for _, e := range bin.BandEnergy {
			totalEnergy += e
		}
	}

	if totalEnergy <= 0 {
		t.Fatal("EvaluatePaths() produced zero total energy, want > 0")
	}
}

func TestEvaluatePaths_DifferentMaterialsDifferentEnergy(t *testing.T) {
	t.Parallel()

	// Trace paths once with a low-absorption scene.
	rtLow := newEvaluatePathsRayTracer(0.05)

	cache, err := rtLow.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	histLow, err := rtLow.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths(low) error = %v", err)
	}

	// Build a high-absorption ray tracer with the same geometry.
	rtHigh := newEvaluatePathsRayTracer(0.9)

	histHigh, err := rtHigh.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths(high) error = %v", err)
	}

	var energyLow, energyHigh float64
	for _, bin := range histLow.Bins {
		for _, e := range bin.BandEnergy {
			energyLow += e
		}
	}

	for _, bin := range histHigh.Bins {
		for _, e := range bin.BandEnergy {
			energyHigh += e
		}
	}

	if energyLow <= 0 {
		t.Fatal("low-absorption energy is zero")
	}

	if energyHigh <= 0 {
		t.Fatal("high-absorption energy is zero")
	}

	if energyHigh >= energyLow {
		t.Fatalf("high-absorption energy (%v) >= low-absorption energy (%v), want less", energyHigh, energyLow)
	}
}
