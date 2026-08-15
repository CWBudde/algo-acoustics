package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// mergedGroupScene builds two adjacent shoebox rooms joined by a floor-standing
// door and returns the merged group as a single-room scene.
func mergedGroupScene(t *testing.T) *scene.Scene {
	t.Helper()

	shoebox := func(originX float64) scene.Room {
		return scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Origin: geometry.Vec3{X: originX},
				Width:  6, Depth: 4, Height: 3,
				WallMaterials: [6]string{"wall", "wall", "wall", "wall", "floor", "ceiling"},
			},
		}
	}

	sc := &scene.Scene{
		Rooms: []scene.Room{shoebox(0), shoebox(6)},
		Portals: []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: 6, Y: 1.4, Z: 0},
				{X: 6, Y: 2.6, Z: 0},
				{X: 6, Y: 2.6, Z: 2.1},
				{X: 6, Y: 1.4, Z: 2.1},
			},
			Material: "door",
			State:    scene.PortalOpen,
		}},
		Materials: map[string]scene.Material{
			"wall":    {Name: "wall", AbsorptionByBand: []float64{0.1}, ScatteringByBand: []float64{0.05}},
			"floor":   {Name: "floor", AbsorptionByBand: []float64{0.05}, ScatteringByBand: []float64{0.05}},
			"ceiling": {Name: "ceiling", AbsorptionByBand: []float64{0.2}, ScatteringByBand: []float64{0.05}},
			"door":    {Name: "door", AbsorptionByBand: []float64{0.08}, SoundReductionIndex: []float64{30}},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 9, Y: 2, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	graph, err := scene.NewAcousticSceneGraph(sc)
	if err != nil {
		t.Fatalf("NewAcousticSceneGraph: %v", err)
	}

	sub, err := graph.GroupScene(0)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	return sub
}

// TestMergedGroupSceneSolvesWithTheMeshImageSourceSolver checks that a room
// group merged across an open portal drops straight into the mesh image-source
// solver. The merged boundary carries coincident partition sheets and is
// intentionally not edge-manifold, so this pins that the solver copes.
func TestMergedGroupSceneSolvesWithTheMeshImageSourceSolver(t *testing.T) {
	t.Parallel()

	sub := mergedGroupScene(t)

	events, err := (ISMSolver{}).Solve(sub, ISMConfig{MaxOrder: 2, BandSpec: acoustics.Octave6})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("the merged group produced no image-source events")
	}

	// Source and receiver are 7 m apart with clear line of sight through the
	// doorway, so the direct sound must be present and arrive first.
	direct := events[0]
	if direct.Kind != ir.EventDirect {
		t.Fatalf("first event kind = %v, want the direct sound", direct.Kind)
	}

	if want := 7.0; direct.DistanceMeters < want-1e-6 || direct.DistanceMeters > want+1e-6 {
		t.Fatalf("direct distance = %v m, want %v m", direct.DistanceMeters, want)
	}
}

// TestMergedGroupSceneSpecularEventsUseTheContributingRoomMaterials guards the
// reason per-triangle materials exist: the merged mesh must not collapse the
// six wall materials onto one.
func TestMergedGroupSceneSpecularEventsUseTheContributingRoomMaterials(t *testing.T) {
	t.Parallel()

	sub := mergedGroupScene(t)

	set := meshMaterials(sub)
	if set.perTriangle == nil {
		t.Fatal("a merged group must carry a per-triangle material table")
	}

	names := map[string]bool{}
	for index := range sub.Room.Mesh.Triangles {
		names[set.At(index).Name] = true
	}

	for _, want := range []string{"wall", "floor", "ceiling"} {
		if !names[want] {
			t.Fatalf("merged group lost the %q material; got %v", want, names)
		}
	}
}
