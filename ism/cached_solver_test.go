package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func assertEventsEqual(t *testing.T, a, b []ir.Event, label string) {
	t.Helper()

	if len(a) != len(b) {
		t.Fatalf("%s: event count mismatch: %d vs %d", label, len(a), len(b))
	}

	for i := range a {
		if a[i].TimeSeconds != b[i].TimeSeconds {
			t.Fatalf("%s: event[%d].TimeSeconds: %v != %v", label, i, a[i].TimeSeconds, b[i].TimeSeconds)
		}

		if a[i].Kind != b[i].Kind {
			t.Fatalf("%s: event[%d].Kind: %v != %v", label, i, a[i].Kind, b[i].Kind)
		}

		if a[i].DistanceMeters != b[i].DistanceMeters {
			t.Fatalf("%s: event[%d].DistanceMeters: %v != %v", label, i, a[i].DistanceMeters, b[i].DistanceMeters)
		}

		if a[i].Amplitude != b[i].Amplitude {
			t.Fatalf("%s: event[%d].Amplitude: %v != %v", label, i, a[i].Amplitude, b[i].Amplitude)
		}

		if len(a[i].BandGain) != len(b[i].BandGain) {
			t.Fatalf("%s: event[%d].BandGain length: %d != %d", label, i, len(a[i].BandGain), len(b[i].BandGain))
		}

		for band := range a[i].BandGain {
			if a[i].BandGain[band] != b[i].BandGain[band] {
				t.Fatalf("%s: event[%d].BandGain[%d]: %v != %v", label, i, band, a[i].BandGain[band], b[i].BandGain[band])
			}
		}
	}
}

func TestCachedISMSolver_ShoeboxMatchesSolveFirstCall(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	reference, err := ISMSolver{}.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("ISMSolver.Solve() error = %v", err)
	}

	cached := &CachedISMSolver{}

	got, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("CachedISMSolver.Solve() error = %v", err)
	}

	assertEventsEqual(t, reference, got, "first call")
}

func TestCachedISMSolver_ShoeboxMaterialChangeReusesCache(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	cached := &CachedISMSolver{}

	_, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("first Solve error = %v", err)
	}

	// Snapshot cached image sources so we can verify they were reused.
	firstCache := cached.shoeboxCache[0]

	// Change material absorption.
	modified := sc
	modified.Materials = map[string]scene.Material{
		"hard": {
			Name:             "hard",
			AbsorptionByBand: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
			ScatteringByBand: []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		},
	}

	got, err := cached.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("second Solve error = %v", err)
	}

	// Cache must be reused (same slice reference).
	if &cached.shoeboxCache[0][0] != &firstCache[0] {
		t.Fatal("material-only change should reuse cached image sources (no reallocation)")
	}

	reference, err := ISMSolver{}.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("reference Solve error = %v", err)
	}

	assertEventsEqual(t, reference, got, "material changed")
}

func TestCachedISMSolver_ShoeboxReceiverMoveReusesCache(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	cached := &CachedISMSolver{}

	_, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("first Solve error = %v", err)
	}

	firstCache := cached.shoeboxCache[0]

	// Move the receiver.
	modified := sc
	modified.Receivers = []scene.Receiver{{
		Position:    geometry.Vec3{X: 6, Y: 3, Z: 2},
		Orientation: geometry.QuatIdentity(),
		Type:        scene.ReceiverOmni,
	}}

	got, err := cached.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("second Solve error = %v", err)
	}

	// Cache must still be reused: receiver moves don't affect image sources.
	if &cached.shoeboxCache[0][0] != &firstCache[0] {
		t.Fatal("receiver move should reuse cached image sources (no reallocation)")
	}

	reference, err := ISMSolver{}.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("reference Solve error = %v", err)
	}

	assertEventsEqual(t, reference, got, "receiver moved")
}

func TestCachedISMSolver_ShoeboxSourceMoveRegenerates(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	cached := &CachedISMSolver{}

	_, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("first Solve error = %v", err)
	}

	firstCache := cached.shoeboxCache[0]

	// Move the source.
	modified := sc
	modified.Sources = []scene.Source{{
		Position:    geometry.Vec3{X: 2, Y: 3, Z: 2},
		Orientation: geometry.QuatIdentity(),
		Directivity: sc.Sources[0].Directivity,
	}}

	got, err := cached.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("second Solve error = %v", err)
	}

	// Cache slice for the moved source must have been regenerated.
	if len(cached.shoeboxCache[0]) != len(firstCache) {
		t.Fatalf("regenerated source cache length %d != original %d (orders differ)",
			len(cached.shoeboxCache[0]), len(firstCache))
	}

	// At least one image source position should differ.
	anyDifferent := false

	for i := range cached.shoeboxCache[0] {
		if cached.shoeboxCache[0][i].Position != firstCache[i].Position {
			anyDifferent = true

			break
		}
	}

	if !anyDifferent {
		t.Fatal("source move should regenerate image source positions")
	}

	reference, err := ISMSolver{}.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("reference Solve error = %v", err)
	}

	assertEventsEqual(t, reference, got, "source moved")
}

func TestCachedISMSolver_ShoeboxRoomChangeRebuilds(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	cached := &CachedISMSolver{}

	_, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("first Solve error = %v", err)
	}

	originalHash := cached.lastRoomHash

	// Change room dimensions.
	modified := sc
	modified.Room.Shoebox = &scene.Shoebox{
		Width:         sc.Room.Shoebox.Width + 2,
		Depth:         sc.Room.Shoebox.Depth,
		Height:        sc.Room.Shoebox.Height,
		WallMaterials: sc.Room.Shoebox.WallMaterials,
	}

	got, err := cached.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("second Solve error = %v", err)
	}

	if cached.lastRoomHash == originalHash {
		t.Fatal("room hash should change after dimension change")
	}

	reference, err := ISMSolver{}.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("reference Solve error = %v", err)
	}

	assertEventsEqual(t, reference, got, "room dimensions changed")
}

func TestCachedISMSolver_ShoeboxMaxOrderChangeRebuilds(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     2,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	cached := &CachedISMSolver{}

	_, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("first Solve error = %v", err)
	}

	firstCount := len(cached.shoeboxCache[0])

	// Increase max order.
	cfg.MaxOrder = 4

	got, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("second Solve error = %v", err)
	}

	if len(cached.shoeboxCache[0]) == firstCount {
		t.Fatal("changing MaxOrder should regenerate the image source list")
	}

	reference, err := ISMSolver{}.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("reference Solve error = %v", err)
	}

	assertEventsEqual(t, reference, got, "max order increased")
}

func TestCachedISMSolver_MeshMatchesSolve(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	cfg := ISMConfig{
		MaxOrder:     2,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	reference, err := ISMSolver{}.Solve(sc, cfg)
	if err != nil {
		t.Fatalf("ISMSolver.Solve() error = %v", err)
	}

	cached := &CachedISMSolver{}

	got, err := cached.Solve(sc, cfg)
	if err != nil {
		t.Fatalf("CachedISMSolver.Solve() error = %v", err)
	}

	assertEventsEqual(t, reference, got, "mesh first call")
}

func TestCachedISMSolver_MeshMaterialChangeReusesCache(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	cfg := ISMConfig{
		MaxOrder:     2,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	cached := &CachedISMSolver{}

	_, err := cached.Solve(sc, cfg)
	if err != nil {
		t.Fatalf("first Solve error = %v", err)
	}

	firstCache := cached.meshCache[0]

	// Change material.
	modified := *sc
	modified.Materials = map[string]scene.Material{
		"plaster": {
			Name:             "plaster",
			AbsorptionByBand: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
		},
	}

	_, err = cached.Solve(&modified, cfg)
	if err != nil {
		t.Fatalf("second Solve error = %v", err)
	}

	if len(cached.meshCache[0]) != len(firstCache) {
		t.Fatal("material-only change should reuse cached mesh image sources")
	}

	if len(firstCache) > 0 && &cached.meshCache[0][0] != &firstCache[0] {
		t.Fatal("material-only change should reuse cached mesh image source slice")
	}
}

func TestCachedISMSolver_Invalidate(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	cfg := ISMConfig{
		MaxOrder:     2,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	cached := &CachedISMSolver{}

	_, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("first Solve error = %v", err)
	}

	if cached.shoeboxCache == nil {
		t.Fatal("expected cache populated after Solve")
	}

	cached.Invalidate()

	if cached.shoeboxCache != nil {
		t.Error("shoebox cache should be nil after Invalidate")
	}

	if cached.lastRoomHash != 0 {
		t.Error("lastRoomHash should be 0 after Invalidate")
	}

	if cached.lastRoomKind != "" {
		t.Error("lastRoomKind should be empty after Invalidate")
	}

	// Next Solve should still produce correct results (full rebuild).
	got, err := cached.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("Solve after Invalidate error = %v", err)
	}

	reference, err := ISMSolver{}.Solve(&sc, cfg)
	if err != nil {
		t.Fatalf("reference Solve error = %v", err)
	}

	assertEventsEqual(t, reference, got, "after Invalidate")
}

func TestCachedISMSolver_NilScene(t *testing.T) {
	t.Parallel()

	cached := &CachedISMSolver{}

	_, err := cached.Solve(nil, ISMConfig{MaxOrder: 1})
	if err == nil {
		t.Fatal("expected error for nil scene")
	}
}
