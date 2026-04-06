package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func testMeshScene() *scene.Scene {
	mesh := geometry.MeshFromBox(geometry.Vec3{}, geometry.Vec3{X: 4, Y: 3, Z: 2.5})

	return &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "plaster",
		},
		Materials: map[string]scene.Material{
			"plaster": {Name: "plaster", AbsorptionByBand: []float64{0.01, 0.02, 0.02, 0.03, 0.04, 0.05}},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3, Y: 1.5, Z: 1.25}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func TestSolveMeshProducesDirectEvent(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	cfg := ISMConfig{
		MaxOrder:     1,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	events, err := solveMesh(sc, cfg)
	if err != nil {
		t.Fatalf("solveMesh: %v", err)
	}

	var direct *ir.Event

	for i := range events {
		if events[i].Kind == ir.EventDirect {
			direct = &events[i]

			break
		}
	}

	if direct == nil {
		t.Fatal("expected a direct event, got none")
	}

	wantDist := sc.Sources[0].Position.Distance(sc.Receivers[0].Position)

	const tolerance = 1e-9
	if diff := direct.DistanceMeters - wantDist; diff > tolerance || diff < -tolerance {
		t.Errorf("direct distance = %v, want %v", direct.DistanceMeters, wantDist)
	}
}

func TestSolveMeshProducesSpecularEvents(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	cfg := ISMConfig{
		MaxOrder:     2,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	events, err := solveMesh(sc, cfg)
	if err != nil {
		t.Fatalf("solveMesh: %v", err)
	}

	specularCount := 0

	for _, e := range events {
		if e.Kind == ir.EventSpecular {
			specularCount++
		}
	}

	if specularCount == 0 {
		t.Fatal("expected specular events at order 2, got none")
	}

	t.Logf("got %d specular events at order 2", specularCount)
}

func TestSolveMeshSpecularEventsHaveBandGain(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	cfg := ISMConfig{
		MaxOrder:     1,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	events, err := solveMesh(sc, cfg)
	if err != nil {
		t.Fatalf("solveMesh: %v", err)
	}

	for _, e := range events {
		if e.Kind != ir.EventSpecular {
			continue
		}

		if len(e.BandGain) != acoustics.Octave6.BandCount() {
			t.Errorf("specular event band gain length = %d, want %d", len(e.BandGain), acoustics.Octave6.BandCount())
		}

		return
	}

	t.Fatal("no specular events found at order 1")
}

func TestSolveMeshEventsSortedByTime(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	events, err := solveMesh(sc, cfg)
	if err != nil {
		t.Fatalf("solveMesh: %v", err)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	for i := 1; i < len(events); i++ {
		if events[i].TimeSeconds < events[i-1].TimeSeconds {
			t.Errorf("events not sorted by time: event[%d].Time=%v > event[%d].Time=%v",
				i-1, events[i-1].TimeSeconds, i, events[i].TimeSeconds)
		}
	}
}
