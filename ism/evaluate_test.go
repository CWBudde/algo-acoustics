package ism

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestEvaluateShoebox_MatchesSolve(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	events1, err := ISMSolver{}.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	sources := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, cfg.MaxOrder)

	events2, err := EvaluateShoebox(sources, &sc, cfg)
	if err != nil {
		t.Fatalf("EvaluateShoebox() error = %v", err)
	}

	if len(events1) != len(events2) {
		t.Fatalf("event count: Solve=%d, EvaluateShoebox=%d", len(events1), len(events2))
	}

	for i := range events1 {
		if events1[i].TimeSeconds != events2[i].TimeSeconds {
			t.Fatalf("event[%d].TimeSeconds: %v != %v", i, events1[i].TimeSeconds, events2[i].TimeSeconds)
		}

		if events1[i].Kind != events2[i].Kind {
			t.Fatalf("event[%d].Kind: %v != %v", i, events1[i].Kind, events2[i].Kind)
		}

		if events1[i].DistanceMeters != events2[i].DistanceMeters {
			t.Fatalf("event[%d].DistanceMeters: %v != %v", i, events1[i].DistanceMeters, events2[i].DistanceMeters)
		}

		if events1[i].Amplitude != events2[i].Amplitude {
			t.Fatalf("event[%d].Amplitude: %v != %v", i, events1[i].Amplitude, events2[i].Amplitude)
		}

		if len(events1[i].BandGain) != len(events2[i].BandGain) {
			t.Fatalf("event[%d].BandGain length: %d != %d", i, len(events1[i].BandGain), len(events2[i].BandGain))
		}

		for b := range events1[i].BandGain {
			if events1[i].BandGain[b] != events2[i].BandGain[b] {
				t.Fatalf("event[%d].BandGain[%d]: %v != %v", i, b, events1[i].BandGain[b], events2[i].BandGain[b])
			}
		}
	}
}

func TestEvaluateShoebox_DifferentMaterials(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     2,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	// Generate sources once from the scene geometry.
	sources := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, cfg.MaxOrder)

	// Evaluate with low absorption.
	lowAbsScene := sc
	lowAbsScene.Materials = map[string]scene.Material{
		"hard": {
			Name:             "hard",
			AbsorptionByBand: []float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
			ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
		},
	}

	lowEvents, err := EvaluateShoebox(sources, &lowAbsScene, cfg)
	if err != nil {
		t.Fatalf("EvaluateShoebox(low) error = %v", err)
	}

	// Evaluate with high absorption.
	highAbsScene := sc
	highAbsScene.Materials = map[string]scene.Material{
		"hard": {
			Name:             "hard",
			AbsorptionByBand: []float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9},
			ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
		},
	}

	highEvents, err := EvaluateShoebox(sources, &highAbsScene, cfg)
	if err != nil {
		t.Fatalf("EvaluateShoebox(high) error = %v", err)
	}

	// Both should have the same number of specular events (same geometry).
	// But high absorption should have lower band gains for specular events.
	lowSpecular := filterSpecular(lowEvents)
	highSpecular := filterSpecular(highEvents)

	if len(lowSpecular) == 0 {
		t.Fatal("expected specular events with low absorption")
	}

	if len(highSpecular) == 0 {
		t.Fatal("expected specular events with high absorption")
	}

	// Sum band gains for all specular events.
	lowSum := sumSpecularBandGains(lowSpecular)
	highSum := sumSpecularBandGains(highSpecular)

	if highSum >= lowSum {
		t.Fatalf("high absorption gain sum (%v) should be less than low absorption (%v)", highSum, lowSum)
	}
}

func TestEvaluateMesh_MatchesSolve(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	cfg := ISMConfig{
		MaxOrder:     2,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	events1, err := ISMSolver{}.Solve(sc, cfg)
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	// Generate mesh image sources the same way solveMesh does.
	imgCfg := MeshISMConfig{
		MaxOrder:    cfg.MaxOrder,
		MaxDistance: meshMaxDistance(sc, acoustics.SpeedOfSound),
	}

	sources := GenerateMeshImageSources(sc.Sources[0].Position, sc.Room.Mesh, imgCfg)

	events2, err := EvaluateMesh(sources, sc, cfg)
	if err != nil {
		t.Fatalf("EvaluateMesh() error = %v", err)
	}

	if len(events1) != len(events2) {
		t.Fatalf("event count: Solve=%d, EvaluateMesh=%d", len(events1), len(events2))
	}

	for i := range events1 {
		if events1[i].TimeSeconds != events2[i].TimeSeconds {
			t.Fatalf("event[%d].TimeSeconds: %v != %v", i, events1[i].TimeSeconds, events2[i].TimeSeconds)
		}

		if events1[i].Kind != events2[i].Kind {
			t.Fatalf("event[%d].Kind: %v != %v", i, events1[i].Kind, events2[i].Kind)
		}

		if events1[i].DistanceMeters != events2[i].DistanceMeters {
			t.Fatalf("event[%d].DistanceMeters: %v != %v", i, events1[i].DistanceMeters, events2[i].DistanceMeters)
		}

		if events1[i].Amplitude != events2[i].Amplitude {
			t.Fatalf("event[%d].Amplitude: %v != %v", i, events1[i].Amplitude, events2[i].Amplitude)
		}

		if len(events1[i].BandGain) != len(events2[i].BandGain) {
			t.Fatalf("event[%d].BandGain length: %d != %d", i, len(events1[i].BandGain), len(events2[i].BandGain))
		}

		for b := range events1[i].BandGain {
			if events1[i].BandGain[b] != events2[i].BandGain[b] {
				t.Fatalf("event[%d].BandGain[%d]: %v != %v", i, b, events1[i].BandGain[b], events2[i].BandGain[b])
			}
		}
	}
}

func TestEvaluateShoebox_NilScene(t *testing.T) {
	t.Parallel()

	_, err := EvaluateShoebox(nil, nil, ISMConfig{})
	if err == nil {
		t.Fatal("expected error for nil scene")
	}
}

func TestEvaluateMesh_NilScene(t *testing.T) {
	t.Parallel()

	_, err := EvaluateMesh(nil, nil, ISMConfig{})
	if err == nil {
		t.Fatal("expected error for nil scene")
	}
}

func filterSpecular(events []ir.Event) []ir.Event {
	var result []ir.Event

	for _, e := range events {
		if e.Kind == ir.EventSpecular {
			result = append(result, e)
		}
	}

	return result
}

func sumSpecularBandGains(events []ir.Event) float64 {
	sum := 0.0

	for _, e := range events {
		for _, g := range e.BandGain {
			sum += math.Abs(g)
		}
	}

	return sum
}
