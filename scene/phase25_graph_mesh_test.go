package scene_test

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// meshRoomAt builds a box-shaped mesh room carrying a single material.
func meshRoomAt(lower, upper geometry.Vec3, material string) scene.Room {
	return scene.Room{
		Kind:         scene.RoomKindMesh,
		Mesh:         geometry.MeshFromBox(lower, upper),
		MeshMaterial: material,
	}
}

// twoMeshRoomScene joins two box-shaped mesh rooms across their whole shared
// wall. The aperture is triangle-aligned by construction: it is exactly the two
// triangles that MeshFromBox emits for that face.
func twoMeshRoomScene(t *testing.T) *scene.Scene {
	t.Helper()

	sc := &scene.Scene{
		Rooms: []scene.Room{
			meshRoomAt(geometry.Vec3Zero, geometry.Vec3{X: 4, Y: 4, Z: 3}, "wall"),
			meshRoomAt(geometry.Vec3{X: 4}, geometry.Vec3{X: 8, Y: 4, Z: 3}, "floor"),
		},
		Portals: []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: 4, Y: 0, Z: 0},
				{X: 4, Y: 4, Z: 0},
				{X: 4, Y: 4, Z: 3},
				{X: 4, Y: 0, Z: 3},
			},
			Material: "door",
			State:    scene.PortalClosed,
		}},
		Materials: graphTestMaterials(),
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 6, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	err := scene.Validate(sc)
	if err != nil {
		t.Fatalf("two mesh-room fixture is invalid: %v", err)
	}

	return sc
}

func TestGroupGeometryMergesMeshRoomsByDroppingApertureTriangles(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, twoMeshRoomScene(t))

	closedID, _ := graph.GroupOf(0)

	closed, err := graph.GroupGeometry(closedID)
	if err != nil {
		t.Fatalf("GroupGeometry (closed): %v", err)
	}

	if got := len(closed.Mesh.Triangles); got != 12 {
		t.Fatalf("closed group has %d triangles, want the 12 of one box", got)
	}

	openPortals(t, graph, 0)

	openID, _ := graph.GroupOf(0)

	merged, err := graph.GroupGeometry(openID)
	if err != nil {
		t.Fatalf("GroupGeometry (open): %v", err)
	}

	// Two boxes of twelve triangles each, minus the two triangles per room
	// that tiled the shared wall.
	if got := len(merged.Mesh.Triangles); got != 20 {
		t.Fatalf("merged group has %d triangles, want 20", got)
	}

	if want := 2 * 4.0 * 4.0 * 3.0; math.Abs(merged.Volume-want) > 1e-9 {
		t.Fatalf("merged volume = %v, want %v", merged.Volume, want)
	}

	// No triangle may remain on the removed partition plane.
	for index, triangle := range merged.Mesh.Triangles {
		if math.Abs(triangle.Centroid().X-4) < 1e-9 {
			t.Fatalf("triangle %d still tiles the opened partition", index)
		}
	}
}

func TestGroupGeometryMergedMeshRoomsAreClosedAndKeepTheirMaterials(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, twoMeshRoomScene(t))
	openPortals(t, graph, 0)

	id, _ := graph.GroupOf(0)

	sub, err := graph.GroupScene(id)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	err = scene.Validate(sub)
	if err != nil {
		t.Fatalf("merged mesh group scene does not validate: %v", err)
	}

	// Each room kept its own material rather than collapsing onto one.
	sawFirst, sawSecond := false, false

	for index, triangle := range sub.Room.Mesh.Triangles {
		name := sub.Room.TriangleMaterialName(index)

		if triangle.Centroid().X < 4 {
			sawFirst = true

			if name != "wall" {
				t.Fatalf("triangle %d in the first room has material %q, want wall", index, name)
			}

			continue
		}

		sawSecond = true

		if name != "floor" {
			t.Fatalf("triangle %d in the second room has material %q, want floor", index, name)
		}
	}

	if !sawFirst || !sawSecond {
		t.Fatal("the merged mesh does not cover both rooms")
	}
}

func TestGroupGeometryRejectsMisalignedMeshAperture(t *testing.T) {
	t.Parallel()

	sc := twoMeshRoomScene(t)
	// Shrink the aperture so it no longer coincides with an edge loop of the
	// authored mesh: the wall triangles now straddle the portal outline.
	sc.Portals[0].Polygon = []geometry.Vec3{
		{X: 4, Y: 1, Z: 0},
		{X: 4, Y: 3, Z: 0},
		{X: 4, Y: 3, Z: 2},
		{X: 4, Y: 1, Z: 2},
	}
	sc.Portals[0].State = scene.PortalOpen

	graph, err := scene.NewAcousticSceneGraph(sc)
	if err != nil {
		t.Fatalf("NewAcousticSceneGraph: %v", err)
	}

	id, _ := graph.GroupOf(0)

	_, err = graph.GroupGeometry(id)
	if err == nil {
		t.Fatal("GroupGeometry accepted a mesh aperture that no triangle loop follows")
	}

	if !strings.Contains(err.Error(), "retriangulate") {
		t.Fatalf("error = %v, want actionable retriangulation guidance", err)
	}
}
