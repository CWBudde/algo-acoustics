package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// groupSceneFixture builds two adjacent shoebox rooms joined by a floor-standing
// door, mirroring examples/scenes/two_room_transmission.json.
func groupSceneFixture(t *testing.T) *scene.Scene {
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

	err := scene.Validate(sc)
	if err != nil {
		t.Fatalf("group scene fixture is invalid: %v", err)
	}

	return sc
}

// TestMergedGroupSceneTracesEndToEnd is the payoff check for the scene graph:
// a group merged across an open portal must drop straight into the existing
// single-room ray tracer. The merged boundary carries coincident partition
// sheets, so it is intentionally not edge-manifold — this pins that
// NewMeshTracer still accepts it, since it treats validation problems as fatal
// and only tolerates warnings.
func TestMergedGroupSceneTracesEndToEnd(t *testing.T) {
	t.Parallel()

	sc := groupSceneFixture(t)

	graph, err := scene.NewAcousticSceneGraph(sc)
	if err != nil {
		t.Fatalf("NewAcousticSceneGraph: %v", err)
	}

	if got := graph.GroupCount(); got != 1 {
		t.Fatalf("GroupCount = %d, want 1 for an open portal", got)
	}

	sub, err := graph.GroupScene(0)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	if sub.Room.Kind != scene.RoomKindMesh {
		t.Fatalf("merged group room kind = %q, want mesh", sub.Room.Kind)
	}

	_, err = NewMeshTracer(sub.Room.Mesh, sceneMeshTriangleMaterials(sub))
	if err != nil {
		t.Fatalf("NewMeshTracer rejected the merged group boundary: %v", err)
	}

	tracer := &RayTracer{
		Config: LaunchConfig{
			NumRays:        4000,
			MaxBounces:     30,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:          sub,
		ReceiverRadius: 0.5,
	}

	histogram, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}

	total := 0.0

	for _, bin := range histogram.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
		}
	}

	// The source is in one room and the receiver in the other, so energy can
	// only arrive through the opened doorway.
	if total <= 0 {
		t.Fatal("no energy reached the receiver through the merged doorway")
	}
}

// TestClosedPortalGroupSceneIsolatesTheRooms confirms the complementary case:
// with the portal closed the two rooms form separate groups, and the receiver's
// group holds no source at all.
func TestClosedPortalGroupSceneIsolatesTheRooms(t *testing.T) {
	t.Parallel()

	sc := groupSceneFixture(t)
	sc.Portals[0].State = scene.PortalClosed

	graph, err := scene.NewAcousticSceneGraph(sc)
	if err != nil {
		t.Fatalf("NewAcousticSceneGraph: %v", err)
	}

	if got := graph.GroupCount(); got != 2 {
		t.Fatalf("GroupCount = %d, want 2 for a closed portal", got)
	}

	receiverGroup, ok := graph.GroupOfPosition(sc.Receivers[0].Position)
	if !ok {
		t.Fatal("the receiver must belong to a group")
	}

	sub, err := graph.GroupScene(receiverGroup)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	if len(sub.Sources) != 0 {
		t.Fatalf("receiver group holds %d sources, want none", len(sub.Sources))
	}

	if sub.Room.Kind != scene.RoomKindShoebox {
		t.Fatalf("an isolated shoebox group must keep the shoebox fast path, got %q", sub.Room.Kind)
	}
}
