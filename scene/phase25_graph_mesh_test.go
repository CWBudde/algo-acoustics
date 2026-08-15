package scene

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
)

// meshRoomAt builds a box-shaped mesh room carrying a single material.
func meshRoomAt(lower, upper geometry.Vec3, material string) Room {
	return Room{
		Kind:         RoomKindMesh,
		Mesh:         geometry.MeshFromBox(lower, upper),
		MeshMaterial: material,
	}
}

// twoMeshRoomScene joins two box-shaped mesh rooms across their whole shared
// wall. The aperture is triangle-aligned by construction: it is exactly the two
// triangles that MeshFromBox emits for that face.
func twoMeshRoomScene(t *testing.T) *Scene {
	t.Helper()

	sc := &Scene{
		Rooms: []Room{
			meshRoomAt(geometry.Vec3Zero, geometry.Vec3{X: 4, Y: 4, Z: 3}, "wall"),
			meshRoomAt(geometry.Vec3{X: 4}, geometry.Vec3{X: 8, Y: 4, Z: 3}, "floor"),
		},
		Portals: []Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: 4, Y: 0, Z: 0},
				{X: 4, Y: 4, Z: 0},
				{X: 4, Y: 4, Z: 3},
				{X: 4, Y: 0, Z: 3},
			},
			Material: "door",
			State:    PortalClosed,
		}},
		Materials: graphTestMaterials(),
		Sources: []Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []Receiver{{
			Position:    geometry.Vec3{X: 6, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	err := Validate(sc)
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

	err = Validate(sub)
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

// TestGroupGeometryOpensATriangularMeshAperture pins the aperture test to the
// polygon rather than its bounding rectangle. The aperture here is one of the
// two triangles of the shared wall, so both triangles have the same bounding
// rectangle and a rectangle-only test would report the neighbour as crossing
// the outline.
func TestGroupGeometryOpensATriangularMeshAperture(t *testing.T) {
	t.Parallel()

	sc := twoMeshRoomScene(t)
	// Exactly the first triangle MeshFromBox emits for the shared wall, wound
	// from room 0 towards room 1.
	sc.Portals[0].Polygon = []geometry.Vec3{
		{X: 4, Y: 0, Z: 0},
		{X: 4, Y: 4, Z: 0},
		{X: 4, Y: 0, Z: 3},
	}
	sc.Portals[0].State = PortalOpen

	graph := newGraph(t, sc)

	id, _ := graph.GroupOf(0)

	merged, err := graph.GroupGeometry(id)
	if err != nil {
		t.Fatalf("GroupGeometry rejected a triangle-aligned aperture: %v", err)
	}

	// Two boxes of twelve triangles, minus the one triangle per room that
	// tiled the aperture.
	if got := len(merged.Mesh.Triangles); got != 22 {
		t.Fatalf("merged group has %d triangles, want 22", got)
	}
}

// TestGroupGeometryRejectsApertureBesideTheMesh covers the branch where no
// triangle is coplanar with the aperture. Portal validation only demands some
// coplanar room triangle, so a polygon on the wall plane but beside the mesh
// reaches it, and accepting it would merge the rooms through a solid wall.
func TestGroupGeometryRejectsApertureBesideTheMesh(t *testing.T) {
	t.Parallel()

	sc := twoMeshRoomScene(t)
	// On the partition plane, but past the far edge of both rooms.
	sc.Portals[0].Polygon = []geometry.Vec3{
		{X: 4, Y: 6, Z: 0},
		{X: 4, Y: 8, Z: 0},
		{X: 4, Y: 8, Z: 2},
		{X: 4, Y: 6, Z: 2},
	}
	sc.Portals[0].State = PortalOpen

	graph, err := NewAcousticSceneGraph(sc)
	if err != nil {
		t.Fatalf("NewAcousticSceneGraph: %v", err)
	}

	id, _ := graph.GroupOf(0)

	_, err = graph.GroupGeometry(id)
	if err == nil {
		t.Fatal("GroupGeometry accepted an aperture that lies beside the mesh")
	}

	if !strings.Contains(err.Error(), "boundary edge loop") {
		t.Fatalf("error = %v, want it to name the missing edge loop", err)
	}
}

// TestGroupGeometryFollowsMaterialReassignment guards the group cache key: the
// cached geometry carries a derived per-triangle material table, so a material
// change must invalidate it even though no geometry moved.
func TestGroupGeometryFollowsMaterialReassignment(t *testing.T) {
	t.Parallel()

	sc := twoMeshRoomScene(t)
	graph := newGraph(t, sc)
	openPortals(t, graph, 0)

	id, _ := graph.GroupOf(0)

	before, err := graph.GroupGeometry(id)
	if err != nil {
		t.Fatalf("GroupGeometry: %v", err)
	}

	if !slices.Contains(before.TriangleMaterials, "floor") {
		t.Fatal("the merged geometry does not carry the second room's material")
	}

	sc.Rooms[1].MeshMaterial = "door"

	after, err := graph.GroupGeometry(id)
	if err != nil {
		t.Fatalf("GroupGeometry after reassignment: %v", err)
	}

	if slices.Contains(after.TriangleMaterials, "floor") {
		t.Fatal("the merged geometry still carries the replaced material")
	}

	if !slices.Contains(after.TriangleMaterials, "door") {
		t.Fatal("the merged geometry does not carry the reassigned material")
	}
}

// prismMesh builds a triangular prism from a footprint in the XY plane. The
// winding is irrelevant here: the overlap probe is parity-based.
func prismMesh(footprint [3]geometry.Vec2, zMin, zMax float64) *geometry.Mesh {
	lower := [3]geometry.Vec3{}
	upper := [3]geometry.Vec3{}

	for index, corner := range footprint {
		lower[index] = geometry.Vec3{X: corner.U, Y: corner.V, Z: zMin}
		upper[index] = geometry.Vec3{X: corner.U, Y: corner.V, Z: zMax}
	}

	mesh := &geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: lower[0], V1: lower[1], V2: lower[2]},
		{V0: upper[0], V1: upper[2], V2: upper[1]},
	}}

	for index := range footprint {
		next := (index + 1) % 3
		mesh.Triangles = append(
			mesh.Triangles,
			geometry.Triangle{V0: lower[index], V1: lower[next], V2: upper[next]},
			geometry.Triangle{V0: lower[index], V1: upper[next], V2: upper[index]},
		)
	}

	return mesh
}

// TestRoomsShareInteriorIgnoresBoundingBoxOverlap covers the narrow phase of
// the group overlap check. Two wedges splitting a box along its diagonal have
// identical bounding boxes but disjoint interiors, and rejecting them would
// rule out every slanted neighbour.
func TestRoomsShareInteriorIgnoresBoundingBoxOverlap(t *testing.T) {
	t.Parallel()

	lower := Room{Kind: RoomKindMesh, Mesh: prismMesh([3]geometry.Vec2{
		{U: 0, V: 0}, {U: 4, V: 0}, {U: 0, V: 4},
	}, 0, 3)}
	upper := Room{Kind: RoomKindMesh, Mesh: prismMesh([3]geometry.Vec2{
		{U: 4, V: 0}, {U: 4, V: 4}, {U: 0, V: 4},
	}, 0, 3)}

	lowerBounds, _ := lower.Bounds()
	upperBounds, _ := upper.Bounds()

	if !boxesOverlap(lowerBounds, upperBounds, groupGeometryTolerance) {
		t.Fatal("the fixture no longer exercises the narrow phase: the boxes are already disjoint")
	}

	if roomsShareInterior(lower, upper) {
		t.Fatal("two wedges meeting along their shared diagonal wall were reported as overlapping")
	}

	shifted := Room{Kind: RoomKindMesh, Mesh: prismMesh([3]geometry.Vec2{
		{U: 1, V: 1}, {U: 4, V: 0}, {U: 0, V: 4},
	}, 0, 3)}

	if !roomsShareInterior(lower, shifted) {
		t.Fatal("two genuinely intersecting wedges were reported as disjoint")
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
	sc.Portals[0].State = PortalOpen

	graph, err := NewAcousticSceneGraph(sc)
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
