package scene_test

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// twoRoomScene mirrors examples/scenes/two_room_transmission.json: two 6 x 4 x 3
// rooms side by side, joined by a floor-standing door on the shared x = 6 wall.
func twoRoomScene(t *testing.T) *scene.Scene {
	t.Helper()

	sc := &scene.Scene{
		Rooms: []scene.Room{
			shoeboxAt(geometry.Vec3Zero, 6, 4, 3),
			shoeboxAt(geometry.Vec3{X: 6}, 6, 4, 3),
		},
		Portals: []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon:     doorOnX(6, 1.4, 2.6, 2.1),
			Material:    "door",
			State:       scene.PortalClosed,
		}},
		Materials: graphTestMaterials(),
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
		t.Fatalf("two-room fixture is invalid: %v", err)
	}

	return sc
}

// TestGroupGeometryCutsApertureFromBothRooms is the guard against cutting the
// doorway out of only one room's wall. If the neighbour's coincident wall
// survived, the merged cavity would carry a solid partition across the opening
// and this ray would stop at x = 6 instead of reaching the far wall.
func TestGroupGeometryCutsApertureFromBothRooms(t *testing.T) {
	t.Parallel()

	sc := twoRoomScene(t)
	graph := newGraph(t, sc)

	ray := geometry.Ray{
		Origin:    geometry.Vec3{X: 2, Y: 2, Z: 1},
		Direction: geometry.Vec3{X: 1},
	}

	closedID, _ := graph.GroupOf(0)

	closedBVH, err := graph.GroupBVH(closedID)
	if err != nil {
		t.Fatalf("GroupBVH (closed): %v", err)
	}

	closedT, _, hit := closedBVH.Intersect(ray)
	if !hit {
		t.Fatal("the ray must hit the closed partition")
	}

	if math.Abs(closedT-4) > 1e-6 {
		t.Fatalf("closed portal: hit at %v m, want the partition at 4 m", closedT)
	}

	openPortals(t, graph, 0)

	openID, _ := graph.GroupOf(0)

	openBVH, err := graph.GroupBVH(openID)
	if err != nil {
		t.Fatalf("GroupBVH (open): %v", err)
	}

	openT, _, hit := openBVH.Intersect(ray)
	if !hit {
		t.Fatal("the ray must still hit the far wall")
	}

	if math.Abs(openT-10) > 1e-6 {
		t.Fatalf("open portal: hit at %v m, want the far wall at 10 m", openT)
	}
}

func TestGroupGeometryHandlesFloorFlushDoorway(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, twoRoomScene(t))
	openPortals(t, graph, 0)

	id, _ := graph.GroupOf(0)

	group, err := graph.GroupGeometry(id)
	if err != nil {
		t.Fatalf("GroupGeometry: %v", err)
	}

	door := struct{ yMin, yMax, zMin, zMax float64 }{1.4, 2.6, 0, 2.1}

	for index, triangle := range group.Mesh.Triangles {
		if triangle.Area() <= 1e-12 {
			t.Fatalf("triangle %d is degenerate", index)
		}

		centroid := triangle.Centroid()
		if math.Abs(centroid.X-6) > 1e-9 {
			continue
		}

		inDoorway := centroid.Y > door.yMin && centroid.Y < door.yMax &&
			centroid.Z > door.zMin && centroid.Z < door.zMax
		if inDoorway {
			t.Fatalf("triangle %d at %+v lies inside the opened doorway", index, centroid)
		}
	}
}

func TestGroupGeometryVolumeEqualsSumOfRoomVolumes(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, officeFloorScene(t))
	openPortals(t, graph, 0, 1, 2)

	id, _ := graph.GroupOf(0)

	group, err := graph.GroupGeometry(id)
	if err != nil {
		t.Fatalf("GroupGeometry: %v", err)
	}

	// Four 4 x 4 x 3 offices merged into one group.
	if want := 4 * 4.0 * 4.0 * 3.0; math.Abs(group.Volume-want) > 1e-9 {
		t.Fatalf("group volume = %v, want %v", group.Volume, want)
	}

	if group.Bounds.Min.X != 0 || group.Bounds.Max.X != 16 || group.Bounds.Max.Y != 4 {
		t.Fatalf("group bounds = %+v, want the union of the four offices", group.Bounds)
	}
}

// TestGroupGeometryIsClosed checks the group-local closedness rule across many
// portal configurations. Coincident partition sheets make edges used four
// times, which is expected; an odd or single use would be a real hole.
func TestGroupGeometryIsClosed(t *testing.T) {
	t.Parallel()

	configs := [][]int{
		{},
		{0},
		{0, 1, 2},
		{6},
		{0, 6},
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	for _, open := range configs {
		graph := newGraph(t, officeFloorScene(t))
		openPortals(t, graph, open...)

		for id := range graph.GroupCount() {
			group, err := graph.GroupGeometry(scene.GroupID(id))
			if err != nil {
				t.Fatalf("open=%v group %d: %v", open, id, err)
			}

			counts := map[[2]geometry.Vec3]int{}

			for _, triangle := range group.Mesh.Triangles {
				for _, pair := range [3][2]geometry.Vec3{
					{triangle.V0, triangle.V1},
					{triangle.V1, triangle.V2},
					{triangle.V2, triangle.V0},
				} {
					key := pair
					if vertexSortsAfter(pair[0], pair[1]) {
						key = [2]geometry.Vec3{pair[1], pair[0]}
					}

					counts[key]++
				}
			}

			for edge, count := range counts {
				if count < 2 || count%2 != 0 {
					t.Fatalf("open=%v group %d: edge %+v used %d times", open, id, edge, count)
				}
			}
		}
	}
}

func vertexSortsAfter(a, b geometry.Vec3) bool {
	if a.X != b.X {
		return a.X > b.X
	}

	if a.Y != b.Y {
		return a.Y > b.Y
	}

	return a.Z > b.Z
}

func TestGroupGeometryRejectsNonRectangularPortal(t *testing.T) {
	t.Parallel()

	sc := twoRoomScene(t)
	// A triangular doorway still passes portal validation (planar, on the
	// shared wall, correctly wound) but cannot be cut by the rectangle-based
	// algorithm, so it must be rejected rather than silently walled shut.
	sc.Portals[0].Polygon = []geometry.Vec3{
		{X: 6, Y: 1.4, Z: 0},
		{X: 6, Y: 2.6, Z: 0},
		{X: 6, Y: 2.0, Z: 2.1},
	}
	sc.Portals[0].State = scene.PortalOpen

	graph := newGraph(t, sc)

	id, _ := graph.GroupOf(0)

	_, err := graph.GroupGeometry(id)
	if err == nil {
		t.Fatal("GroupGeometry accepted a non-rectangular portal")
	}

	if !strings.Contains(err.Error(), "not an axis-aligned rectangle") {
		t.Fatalf("error = %v, want it to name the rectangle restriction", err)
	}
}

func TestGroupGeometryRejectsOverlappingRooms(t *testing.T) {
	t.Parallel()

	sc := twoRoomScene(t)
	sc.Rooms[1].Shoebox.Origin = geometry.Vec3{X: 5}
	sc.Portals[0].State = scene.PortalOpen

	graph, err := scene.NewAcousticSceneGraph(sc)
	if err != nil {
		// Portal validation may already reject the moved wall, which is an
		// equally good outcome.
		return
	}

	id, _ := graph.GroupOf(0)

	_, err = graph.GroupGeometry(id)
	if err == nil {
		t.Fatal("GroupGeometry accepted overlapping rooms")
	}
}

func TestGroupGeometryCachesUntilTheGroupChanges(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, officeFloorScene(t))

	// Room 7 sits at the far corner, untouched by a door between rooms 0 and 1.
	farID, _ := graph.GroupOf(7)

	before, err := graph.GroupGeometry(farID)
	if err != nil {
		t.Fatalf("GroupGeometry: %v", err)
	}

	openPortals(t, graph, 0)

	farID, _ = graph.GroupOf(7)

	after, err := graph.GroupGeometry(farID)
	if err != nil {
		t.Fatalf("GroupGeometry: %v", err)
	}

	if before != after {
		t.Fatal("an unrelated portal change must not rebuild a distant group's geometry")
	}
}

func TestGroupSceneFastPathPreservesShoebox(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, officeFloorScene(t))

	id, _ := graph.GroupOf(3)

	sub, err := graph.GroupScene(id)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	if sub.Room.Kind != scene.RoomKindShoebox || sub.Room.Shoebox == nil {
		t.Fatalf("group scene room kind = %q, want the shoebox fast path", sub.Room.Kind)
	}

	if sub.Room.Shoebox.WallMaterials[4] != "floor" {
		t.Fatalf("shoebox wall materials were not preserved: %+v", sub.Room.Shoebox.WallMaterials)
	}

	if len(sub.Rooms) != 0 || len(sub.Portals) != 0 {
		t.Fatal("a group scene must not carry the multi-room representation")
	}
}

func TestGroupSceneMergedRoomIsAValidMeshRoom(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, twoRoomScene(t))
	openPortals(t, graph, 0)

	id, _ := graph.GroupOf(0)

	sub, err := graph.GroupScene(id)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	if sub.Room.Kind != scene.RoomKindMesh || sub.Room.Mesh == nil {
		t.Fatalf("merged group room kind = %q, want a mesh room", sub.Room.Kind)
	}

	if len(sub.Room.TriangleMaterials) != len(sub.Room.Mesh.Triangles) {
		t.Fatalf("triangle material count = %d, want %d",
			len(sub.Room.TriangleMaterials), len(sub.Room.Mesh.Triangles))
	}

	// The merged scene must survive validation, since every downstream engine
	// validates before simulating.
	err = scene.Validate(sub)
	if err != nil {
		t.Fatalf("merged group scene does not validate: %v", err)
	}

	// Source and receiver both lie in the merged group.
	if len(sub.Sources) != 1 || len(sub.Receivers) != 1 {
		t.Fatalf("group scene has %d sources and %d receivers, want one of each",
			len(sub.Sources), len(sub.Receivers))
	}
}

func TestGroupSceneAssignsPerWallTriangleMaterials(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, twoRoomScene(t))
	openPortals(t, graph, 0)

	id, _ := graph.GroupOf(0)

	sub, err := graph.GroupScene(id)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	// Merging must not collapse the six wall materials onto one. Floor
	// triangles sit at z = 0 with an upward normal, ceiling triangles at z = 3.
	sawFloor, sawCeiling, sawWall := false, false, false

	for index, triangle := range sub.Room.Mesh.Triangles {
		name := sub.Room.TriangleMaterialName(index)
		centroid := triangle.Centroid()

		switch {
		case math.Abs(centroid.Z) < 1e-9:
			sawFloor = true

			if name != "floor" {
				t.Fatalf("triangle %d on the floor has material %q, want floor", index, name)
			}
		case math.Abs(centroid.Z-3) < 1e-9:
			sawCeiling = true

			if name != "ceiling" {
				t.Fatalf("triangle %d on the ceiling has material %q, want ceiling", index, name)
			}
		default:
			sawWall = true

			if name != "wall" {
				t.Fatalf("triangle %d on a wall has material %q, want wall", index, name)
			}
		}
	}

	if !sawFloor || !sawCeiling || !sawWall {
		t.Fatalf("merged mesh did not cover all surface kinds: floor=%v ceiling=%v wall=%v",
			sawFloor, sawCeiling, sawWall)
	}
}

func TestGroupOfPositionResolvesDoorwayAmbiguityWithinAGroup(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, twoRoomScene(t))

	// A point exactly on the shared wall belongs to both rooms. Scene.RoomIndexAt
	// reports it as ambiguous, but once the two rooms share a group the point is
	// unambiguous at group level.
	inDoorway := geometry.Vec3{X: 6, Y: 2, Z: 1}

	if _, ok := graph.Scene().RoomIndexAt(inDoorway); ok {
		t.Fatal("the fixture no longer exercises the ambiguous-boundary case")
	}

	if _, ok := graph.GroupOfPosition(inDoorway); ok {
		t.Fatal("a doorway point between two separate groups must stay ambiguous")
	}

	openPortals(t, graph, 0)

	id, ok := graph.GroupOfPosition(inDoorway)
	if !ok {
		t.Fatal("a doorway point inside one group must resolve")
	}

	if want, _ := graph.GroupOf(0); id != want {
		t.Fatalf("doorway resolved to group %d, want %d", id, want)
	}
}

func TestSingleRoomSceneFormsOneGroup(t *testing.T) {
	t.Parallel()

	sc := &scene.Scene{
		Room:      shoeboxAt(geometry.Vec3Zero, 6, 4, 3),
		Materials: graphTestMaterials(),
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 4, Y: 2, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	graph := newGraph(t, sc)
	if got := graph.GroupCount(); got != 1 {
		t.Fatalf("GroupCount = %d, want 1 for a single-room scene", got)
	}

	sub, err := graph.GroupScene(0)
	if err != nil {
		t.Fatalf("GroupScene: %v", err)
	}

	if sub.Room.Kind != scene.RoomKindShoebox {
		t.Fatalf("single-room group kind = %q, want shoebox", sub.Room.Kind)
	}
}
