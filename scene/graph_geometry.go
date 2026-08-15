package scene

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// groupGeometryTolerance is the length tolerance used when matching portal
// polygons to room faces and when cutting apertures. It matches the tolerance
// portal validation already applies.
const groupGeometryTolerance = portalGeometryTolerance

// GroupGeometry is the merged boundary of a room group: one triangle soup
// covering every member room, with the apertures of the open portals inside the
// group cut away so the group forms a single connected cavity.
type GroupGeometry struct {
	Mesh              *geometry.Mesh
	TriangleMaterials []string
	// RoomOfTriangle records which member room contributed each triangle.
	RoomOfTriangle []int
	Volume         float64
	Bounds         geometry.Box
	// Hash identifies the rooms and portals this geometry was built from.
	Hash uint64
}

// GroupGeometry returns the merged boundary of a room group, building and
// caching it on first use. The cache is keyed on the hash of the contributing
// rooms and portals, so a portal change elsewhere in the building leaves it
// warm.
func (g *AcousticSceneGraph) GroupGeometry(id GroupID) (*GroupGeometry, error) {
	rooms, ok := g.GroupRooms(id)
	if !ok {
		return nil, fmt.Errorf("group %d does not exist", id)
	}

	hash := g.scene.RoomGroupHash(rooms)

	g.cacheMu.RLock()
	cached, ok := g.geomCache[hash]
	g.cacheMu.RUnlock()

	if ok {
		return cached, nil
	}

	built, err := g.buildGroupGeometry(id, rooms, hash)
	if err != nil {
		return nil, err
	}

	g.cacheMu.Lock()
	g.geomCache[hash] = built
	g.cacheMu.Unlock()

	return built, nil
}

// GroupBVH returns a bounding volume hierarchy over the group's merged
// boundary, cached alongside the geometry.
func (g *AcousticSceneGraph) GroupBVH(id GroupID) (*geometry.BVHNode, error) {
	group, err := g.GroupGeometry(id)
	if err != nil {
		return nil, err
	}

	g.cacheMu.RLock()
	cached, ok := g.bvhCache[group.Hash]
	g.cacheMu.RUnlock()

	if ok {
		return cached, nil
	}

	bvh := geometry.BuildBVH(group.Mesh)
	if bvh == nil {
		return nil, fmt.Errorf("group %d has no triangles to index", id)
	}

	g.cacheMu.Lock()
	g.bvhCache[group.Hash] = bvh
	g.cacheMu.Unlock()

	return bvh, nil
}

func (g *AcousticSceneGraph) buildGroupGeometry(id GroupID, rooms []int, hash uint64) (*GroupGeometry, error) {
	err := g.checkGroupRoomsDoNotOverlap(rooms)
	if err != nil {
		return nil, err
	}

	apertures := g.groupApertures(rooms)
	built := &GroupGeometry{Mesh: &geometry.Mesh{}, Hash: hash}

	for _, roomIndex := range rooms {
		room, ok := g.scene.RoomAt(roomIndex)
		if !ok {
			continue
		}

		err := built.appendRoom(*room, roomIndex, apertures[roomIndex])
		if err != nil {
			return nil, err
		}
	}

	if len(built.Mesh.Triangles) == 0 {
		return nil, fmt.Errorf("group %d produced no boundary triangles", id)
	}

	err = groupMeshIsClosed(built.Mesh)
	if err != nil {
		return nil, fmt.Errorf("group %d boundary is not closed: %w", id, err)
	}

	built.Volume, built.Bounds, err = g.groupExtent(rooms)
	if err != nil {
		return nil, fmt.Errorf("group %d: %w", id, err)
	}

	return built, nil
}

// groupApertures collects, per room, the polygons of the open portals whose two
// rooms both belong to the group.
//
// A portal always yields an entry for BOTH of its rooms. Cutting the doorway
// out of only one side would leave the neighbour's coincident wall intact, so
// the merged cavity would carry a solid partition straight across the opening
// with a paper-thin void behind it, and no ray would ever cross.
//
// Closed portals contribute nothing. They remain solid wall carrying the wall
// material, and their transmission is applied later by the portal filter of the
// propagation path; cutting them here as well would double-count.
func (g *AcousticSceneGraph) groupApertures(rooms []int) map[int][][]geometry.Vec3 {
	inGroup := make(map[int]bool, len(rooms))
	for _, roomIndex := range rooms {
		inGroup[roomIndex] = true
	}

	apertures := make(map[int][][]geometry.Vec3, len(rooms))

	for _, portal := range g.scene.Portals {
		if portal.State != PortalOpen {
			continue
		}

		if !inGroup[portal.RoomIndices[0]] || !inGroup[portal.RoomIndices[1]] {
			continue
		}

		for _, roomIndex := range portal.RoomIndices {
			apertures[roomIndex] = append(apertures[roomIndex], portal.Polygon)
		}
	}

	return apertures
}

func (g *AcousticSceneGraph) checkGroupRoomsDoNotOverlap(rooms []int) error {
	for i, first := range rooms {
		firstRoom, ok := g.scene.RoomAt(first)
		if !ok {
			continue
		}

		firstBounds, ok := firstRoom.Bounds()
		if !ok {
			continue
		}

		for _, second := range rooms[i+1:] {
			secondRoom, ok := g.scene.RoomAt(second)
			if !ok {
				continue
			}

			secondBounds, ok := secondRoom.Bounds()
			if !ok {
				continue
			}

			if boxesOverlap(firstBounds, secondBounds, groupGeometryTolerance) {
				return fmt.Errorf("rooms %d and %d overlap and cannot share a group", first, second)
			}
		}
	}

	return nil
}

// groupExtent returns the group's physical volume and bounds. The volume is the
// sum of the member room volumes rather than the merged mesh's enclosed volume:
// two rooms sharing a partition contribute two coincident sheets, so the merged
// mesh is intentionally not edge-manifold and Mesh.EnclosedVolume rejects it.
func (g *AcousticSceneGraph) groupExtent(rooms []int) (float64, geometry.Box, error) {
	volume := 0.0
	bounds := geometry.Box{}
	seen := false

	for _, roomIndex := range rooms {
		room, ok := g.scene.RoomAt(roomIndex)
		if !ok {
			continue
		}

		roomVolume, ok := room.Volume()
		if !ok {
			return 0, geometry.Box{}, fmt.Errorf("room %d has no derivable volume", roomIndex)
		}

		roomBounds, ok := room.Bounds()
		if !ok {
			return 0, geometry.Box{}, fmt.Errorf("room %d has no derivable bounds", roomIndex)
		}

		volume += roomVolume

		if !seen {
			bounds = roomBounds
			seen = true

			continue
		}

		bounds = geometry.NewBox(
			geometry.Vec3{
				X: math.Min(bounds.Min.X, roomBounds.Min.X),
				Y: math.Min(bounds.Min.Y, roomBounds.Min.Y),
				Z: math.Min(bounds.Min.Z, roomBounds.Min.Z),
			},
			geometry.Vec3{
				X: math.Max(bounds.Max.X, roomBounds.Max.X),
				Y: math.Max(bounds.Max.Y, roomBounds.Max.Y),
				Z: math.Max(bounds.Max.Z, roomBounds.Max.Z),
			},
		)
	}

	return volume, bounds, nil
}

func (b *GroupGeometry) appendRoom(room Room, roomIndex int, apertures [][]geometry.Vec3) error {
	switch room.Kind {
	case RoomKindShoebox:
		return b.appendShoeboxRoom(room, roomIndex, apertures)
	case RoomKindMesh:
		return b.appendMeshRoom(room, roomIndex, apertures)
	default:
		return fmt.Errorf("room %d has an unsupported kind %q", roomIndex, room.Kind)
	}
}

func (b *GroupGeometry) appendShoeboxRoom(room Room, roomIndex int, apertures [][]geometry.Vec3) error {
	if room.Shoebox == nil {
		return fmt.Errorf("room %d is a shoebox without geometry", roomIndex)
	}

	bounds := room.Shoebox.Bounds()

	for wall, face := range shoeboxFaces(bounds) {
		holes, err := faceHoles(face, apertures, roomIndex)
		if err != nil {
			return err
		}

		triangles, err := geometry.CutRectangularHoles(face.frame, face.rect, holes, groupGeometryTolerance)
		if err != nil {
			return fmt.Errorf("cut wall %d of room %d: %w", wall, roomIndex, err)
		}

		b.append(triangles, room.Shoebox.WallMaterials[wall], roomIndex)
	}

	return nil
}

// appendMeshRoom copies a mesh room's triangles, dropping those that tile an
// open portal's aperture.
//
// Arbitrary mesh clipping is deliberately out of scope: the aperture must be
// triangle-aligned in the authored mesh, so opening a portal is exactly a
// deletion. A mesh whose triangles straddle the portal outline is rejected with
// an actionable message rather than silently mis-simulated.
func (b *GroupGeometry) appendMeshRoom(room Room, roomIndex int, apertures [][]geometry.Vec3) error {
	if room.Mesh == nil {
		return fmt.Errorf("room %d is a mesh room without geometry", roomIndex)
	}

	dropped := make([]bool, len(room.Mesh.Triangles))

	for _, polygon := range apertures {
		err := markApertureTriangles(room, roomIndex, polygon, dropped)
		if err != nil {
			return err
		}
	}

	for index, triangle := range room.Mesh.Triangles {
		if dropped[index] {
			continue
		}

		b.appendOne(triangle, room.TriangleMaterialName(index), roomIndex)
	}

	return nil
}

// markApertureTriangles flags the triangles tiling one portal aperture. It
// requires their combined area to match the portal area, which is what pins the
// aperture to an edge loop in the authored mesh.
func markApertureTriangles(room Room, roomIndex int, polygon []geometry.Vec3, dropped []bool) error {
	if len(polygon) < 3 {
		return fmt.Errorf("portal polygon of room %d has fewer than three vertices", roomIndex)
	}

	normal := geometry.NewPlaneFromPointNormal(polygon[0], polygonNormal(polygon))
	frame := geometry.NewPlaneFrame(polygon[0], normal.Normal)
	projected := make([]geometry.Vec2, 0, len(polygon))

	for _, vertex := range polygon {
		projected = append(projected, frame.To2D(vertex))
	}

	outline := geometry.BoundingRect2(projected)
	covered := 0.0
	straddling := 0

	for index, triangle := range room.Mesh.Triangles {
		if dropped[index] {
			continue
		}

		if !triangleTouchesAperture(frame, outline, triangle) {
			continue
		}

		if !triangleInsidePolygon(frame, projected, triangle) {
			// Coplanar with the aperture and overlapping it, but not wholly
			// inside: this triangle crosses the outline.
			straddling++

			continue
		}

		dropped[index] = true
		covered += triangle.Area()
	}

	// Nothing coplanar reaches the aperture, so the outline is already a hole
	// in the authored mesh. That is the other legitimate way to model an
	// opening, and there is nothing to delete.
	if covered == 0 && straddling == 0 {
		return nil
	}

	portalArea := polygonArea(polygon, normal.Normal)
	if straddling > 0 || math.Abs(covered-portalArea) > math.Max(1e-6, portalArea*1e-6) {
		return fmt.Errorf(
			"portal aperture does not align with the triangles of mesh room %d: %d triangles cross the outline and %.9g m2 of %.9g m2 is tiled exactly; retriangulate so the portal outline is an edge loop",
			roomIndex, straddling, covered, portalArea,
		)
	}

	return nil
}

// triangleTouchesAperture reports whether a triangle lies on the aperture plane
// and its projection shares positive area with the aperture outline.
func triangleTouchesAperture(frame geometry.PlaneFrame, outline geometry.Rect2, triangle geometry.Triangle) bool {
	vertices := [3]geometry.Vec3{triangle.V0, triangle.V1, triangle.V2}
	projected := make([]geometry.Vec2, 0, 3)

	for _, vertex := range vertices {
		if math.Abs(frame.Distance(vertex)) > groupGeometryTolerance {
			return false
		}

		projected = append(projected, frame.To2D(vertex))
	}

	return geometry.BoundingRect2(projected).Overlaps(outline, groupGeometryTolerance)
}

func (b *GroupGeometry) append(triangles []geometry.Triangle, material string, roomIndex int) {
	for _, triangle := range triangles {
		b.appendOne(triangle, material, roomIndex)
	}
}

func (b *GroupGeometry) appendOne(triangle geometry.Triangle, material string, roomIndex int) {
	b.Mesh.Triangles = append(b.Mesh.Triangles, triangle)
	b.TriangleMaterials = append(b.TriangleMaterials, material)
	b.RoomOfTriangle = append(b.RoomOfTriangle, roomIndex)
}

// shoeboxFace pairs a wall's plane basis with its extent in that basis.
type shoeboxFace struct {
	frame geometry.PlaneFrame
	rect  geometry.Rect2
}

// shoeboxFaces returns the six walls in the order -X, +X, -Y, +Y, -Z, +Z, each
// with an inward normal. This matches both geometry.MeshFromBox and the wall
// index order that Shoebox.WallMaterials uses.
func shoeboxFaces(bounds geometry.Box) [6]shoeboxFace {
	corners := [6][4]geometry.Vec3{
		{ // -X
			{X: bounds.Min.X, Y: bounds.Min.Y, Z: bounds.Min.Z},
			{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Min.Z},
			{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Max.Z},
			{X: bounds.Min.X, Y: bounds.Min.Y, Z: bounds.Max.Z},
		},
		{ // +X
			{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Max.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Max.Y, Z: bounds.Max.Z},
			{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Max.Z},
		},
		{ // -Y
			{X: bounds.Min.X, Y: bounds.Min.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Max.Z},
			{X: bounds.Min.X, Y: bounds.Min.Y, Z: bounds.Max.Z},
		},
		{ // +Y
			{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Max.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Max.Y, Z: bounds.Max.Z},
			{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Max.Z},
		},
		{ // -Z
			{X: bounds.Min.X, Y: bounds.Min.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Min.Z},
			{X: bounds.Max.X, Y: bounds.Max.Y, Z: bounds.Min.Z},
			{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Min.Z},
		},
		{ // +Z
			{X: bounds.Min.X, Y: bounds.Min.Y, Z: bounds.Max.Z},
			{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Max.Z},
			{X: bounds.Max.X, Y: bounds.Max.Y, Z: bounds.Max.Z},
			{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Max.Z},
		},
	}

	inward := [6]geometry.Vec3{
		{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1},
	}

	var faces [6]shoeboxFace

	for wall := range faces {
		frame := geometry.NewPlaneFrame(corners[wall][0], inward[wall])
		projected := make([]geometry.Vec2, 0, 4)

		for _, corner := range corners[wall] {
			projected = append(projected, frame.To2D(corner))
		}

		rect, _ := geometry.Rect2FromPolygon(projected, groupGeometryTolerance)
		faces[wall] = shoeboxFace{frame: frame, rect: rect}
	}

	return faces
}

// faceHoles returns the apertures lying on a wall, projected into its basis.
// A portal that sits on the wall plane but is not an axis-aligned rectangle in
// that basis is rejected: the cut algorithm is rectangle-based by design, and
// silently ignoring such a portal would leave the doorway walled shut.
func faceHoles(face shoeboxFace, apertures [][]geometry.Vec3, roomIndex int) ([]geometry.Rect2, error) {
	var holes []geometry.Rect2

	for _, polygon := range apertures {
		if !polygonOnFacePlane(face, polygon) {
			continue
		}

		hole, ok := geometry.RectangleFromCoplanarPolygon(face.frame, polygon, groupGeometryTolerance)
		if !ok {
			return nil, fmt.Errorf(
				"portal on room %d is not an axis-aligned rectangle in its wall plane; only rectangular portals can be opened",
				roomIndex,
			)
		}

		if !face.rect.Contains(hole, groupGeometryTolerance) {
			continue
		}

		holes = append(holes, hole)
	}

	return holes, nil
}

func polygonOnFacePlane(face shoeboxFace, polygon []geometry.Vec3) bool {
	for _, vertex := range polygon {
		if math.Abs(face.frame.Distance(vertex)) > groupGeometryTolerance {
			return false
		}
	}

	return len(polygon) > 0
}

// triangleInsidePolygon reports whether all three vertices of a triangle lie on
// the polygon's plane and within its outline.
func triangleInsidePolygon(frame geometry.PlaneFrame, polygon []geometry.Vec2, triangle geometry.Triangle) bool {
	for _, vertex := range [3]geometry.Vec3{triangle.V0, triangle.V1, triangle.V2} {
		if math.Abs(frame.Distance(vertex)) > groupGeometryTolerance {
			return false
		}

		if !pointInPolygon2D(polygon, frame.To2D(vertex), groupGeometryTolerance) {
			return false
		}
	}

	return true
}

// pointInPolygon2D is a winding-free crossing test that also accepts points on
// the outline, so triangles sharing the aperture border still count as inside.
func pointInPolygon2D(polygon []geometry.Vec2, point geometry.Vec2, eps float64) bool {
	inside := false

	for index, current := range polygon {
		next := polygon[(index+1)%len(polygon)]

		if pointOnSegment2D(current, next, point, eps) {
			return true
		}

		if (current.V > point.V) == (next.V > point.V) {
			continue
		}

		crossU := current.U + (point.V-current.V)/(next.V-current.V)*(next.U-current.U)
		if point.U < crossU {
			inside = !inside
		}
	}

	return inside
}

func pointOnSegment2D(a, b, point geometry.Vec2, eps float64) bool {
	du, dv := b.U-a.U, b.V-a.V
	length := math.Hypot(du, dv)

	if length <= eps {
		return math.Hypot(point.U-a.U, point.V-a.V) <= eps
	}

	cross := (point.U-a.U)*dv - (point.V-a.V)*du
	if math.Abs(cross)/length > eps {
		return false
	}

	dot := (point.U-a.U)*du + (point.V-a.V)*dv

	return dot >= -eps*length && dot <= length*length+eps*length
}

func polygonNormal(polygon []geometry.Vec3) geometry.Vec3 {
	origin := polygon[0]
	for index := 1; index+1 < len(polygon); index++ {
		normal := polygon[index].Sub(origin).Cross(polygon[index+1].Sub(origin))
		if normal.Norm() > groupGeometryTolerance {
			return normal.Normalize()
		}
	}

	return geometry.Vec3Zero
}

func polygonArea(polygon []geometry.Vec3, normal geometry.Vec3) float64 {
	doubleArea := 0.0

	for index, vertex := range polygon {
		next := polygon[(index+1)%len(polygon)]
		doubleArea += vertex.Cross(next).Dot(normal)
	}

	return math.Abs(doubleArea) * 0.5
}

func boxesOverlap(a, b geometry.Box, eps float64) bool {
	return math.Min(a.Max.X, b.Max.X)-math.Max(a.Min.X, b.Min.X) > eps &&
		math.Min(a.Max.Y, b.Max.Y)-math.Max(a.Min.Y, b.Min.Y) > eps &&
		math.Min(a.Max.Z, b.Max.Z)-math.Max(a.Min.Z, b.Min.Z) > eps
}

// groupMeshIsClosed checks that the merged boundary has no open edge.
//
// It cannot use Mesh.Validate's manifold rule. Two rooms sharing a partition
// contribute two coincident sheets with opposing normals — physically right,
// since each side carries its own material — so edges on that partition are
// used four times. The group-local rule is therefore that every undirected edge
// is used an even number of times and at least twice.
func groupMeshIsClosed(mesh *geometry.Mesh) error {
	type edgeKey struct{ a, b geometry.Vec3 }

	counts := make(map[edgeKey]int, len(mesh.Triangles)*3)

	for _, triangle := range mesh.Triangles {
		for _, pair := range [3][2]geometry.Vec3{
			{triangle.V0, triangle.V1},
			{triangle.V1, triangle.V2},
			{triangle.V2, triangle.V0},
		} {
			key := edgeKey{a: pair[0], b: pair[1]}
			if vec3SortsAfter(pair[0], pair[1]) {
				key = edgeKey{a: pair[1], b: pair[0]}
			}

			counts[key]++
		}
	}

	open := 0

	for _, count := range counts {
		if count < 2 || count%2 != 0 {
			open++
		}
	}

	if open > 0 {
		return fmt.Errorf("%d boundary edges are open", open)
	}

	return nil
}

func vec3SortsAfter(a, b geometry.Vec3) bool {
	if a.X != b.X {
		return a.X > b.X
	}

	if a.Y != b.Y {
		return a.Y > b.Y
	}

	return a.Z > b.Z
}
