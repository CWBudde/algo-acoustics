package geometry

import (
	"math"
	"testing"
)

const faceCutEps = 1e-9

// coveredArea sums the triangle areas, which must equal the face area minus the
// hole areas whenever the cut succeeded.
func coveredArea(triangles []Triangle) float64 {
	total := 0.0
	for _, triangle := range triangles {
		total += triangle.Area()
	}

	return total
}

func rectArea(r Rect2) float64 {
	return (r.UMax - r.UMin) * (r.VMax - r.VMin)
}

func TestCutRectangularHolesCoversFaceMinusHoles(t *testing.T) {
	t.Parallel()

	face := Rect2{UMax: 4, VMax: 3}

	tests := []struct {
		name  string
		holes []Rect2
	}{
		{
			name:  "no holes",
			holes: nil,
		},
		{
			name:  "hole strictly interior",
			holes: []Rect2{{UMin: 1, VMin: 1, UMax: 2, VMax: 2}},
		},
		{
			// The canonical case: a door standing on the floor, matching
			// examples/scenes/two_room_transmission.json.
			name:  "hole flush with one edge",
			holes: []Rect2{{UMin: 1, VMin: 0, UMax: 2, VMax: 2.1}},
		},
		{
			name:  "hole flush with two edges",
			holes: []Rect2{{UMin: 0, VMin: 0, UMax: 2, VMax: 2}},
		},
		{
			name:  "hole flush with three edges",
			holes: []Rect2{{UMin: 0, VMin: 0, UMax: 4, VMax: 2}},
		},
		{
			name:  "two disjoint holes",
			holes: []Rect2{{UMin: 0.5, VMin: 0, UMax: 1.5, VMax: 2}, {UMin: 2.5, VMin: 0, UMax: 3.5, VMax: 2}},
		},
		{
			name:  "two holes sharing an edge",
			holes: []Rect2{{UMin: 1, VMin: 0, UMax: 2, VMax: 2}, {UMin: 2, VMin: 0, UMax: 3, VMax: 2}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})

			triangles, err := CutRectangularHoles(frame, face, test.holes, faceCutEps)
			if err != nil {
				t.Fatalf("CutRectangularHoles: %v", err)
			}

			want := rectArea(face)
			for _, hole := range test.holes {
				want -= rectArea(hole)
			}

			if got := coveredArea(triangles); math.Abs(got-want) > 1e-9 {
				t.Fatalf("covered area = %v, want %v", got, want)
			}

			for index, triangle := range triangles {
				if triangle.Area() <= faceCutEps {
					t.Fatalf("triangle %d is degenerate", index)
				}

				if triangle.Normal().Sub(frame.Normal).Norm() > 1e-9 {
					t.Fatalf("triangle %d normal = %+v, want %+v", index, triangle.Normal(), frame.Normal)
				}
			}
		})
	}
}

func TestCutRectangularHolesFullCoverageDeletesFace(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})
	face := Rect2{UMax: 4, VMax: 3}

	triangles, err := CutRectangularHoles(frame, face, []Rect2{face}, faceCutEps)
	if err != nil {
		t.Fatalf("CutRectangularHoles: %v", err)
	}

	if len(triangles) != 0 {
		t.Fatalf("got %d triangles, want none for a hole covering the whole face", len(triangles))
	}
}

func TestCutRectangularHolesEmitsGridWithoutMerging(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})
	face := Rect2{UMax: 4, VMax: 3}

	// An interior hole splits the face into a 3x3 grid whose center cell is
	// the hole, leaving eight cells and therefore sixteen triangles. Merging
	// cells would reduce this count but would introduce T-junctions.
	triangles, err := CutRectangularHoles(frame, face, []Rect2{{UMin: 1, VMin: 1, UMax: 2, VMax: 2}}, faceCutEps)
	if err != nil {
		t.Fatalf("CutRectangularHoles: %v", err)
	}

	if len(triangles) != 16 {
		t.Fatalf("got %d triangles, want 16 (eight grid cells)", len(triangles))
	}
}

func TestCutRectangularHolesFloorFlushDoorwayLeavesNoTriangleInTheOpening(t *testing.T) {
	t.Parallel()

	// Reproduces the shipped fixture: a 6 x 3 wall with a door from the floor
	// up to 2.1 m. A ring triangulation assuming a strictly interior hole
	// would fail here.
	frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})
	face := Rect2{UMax: 4, VMax: 3}
	door := Rect2{UMin: 1.4, VMin: 0, UMax: 2.6, VMax: 2.1}

	triangles, err := CutRectangularHoles(frame, face, []Rect2{door}, faceCutEps)
	if err != nil {
		t.Fatalf("CutRectangularHoles: %v", err)
	}

	for index, triangle := range triangles {
		centroid := frame.To2D(triangle.Centroid())
		if door.ContainsPoint(centroid, -faceCutEps) {
			t.Fatalf("triangle %d centroid %+v lies inside the doorway", index, centroid)
		}
	}

	want := rectArea(face) - rectArea(door)
	if got := coveredArea(triangles); math.Abs(got-want) > 1e-9 {
		t.Fatalf("covered area = %v, want %v", got, want)
	}
}

func TestCutRectangularHolesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})
	face := Rect2{UMax: 4, VMax: 3}

	tests := []struct {
		name  string
		face  Rect2
		holes []Rect2
	}{
		{
			name: "degenerate face",
			face: Rect2{UMax: 4},
		},
		{
			name:  "hole outside the face",
			face:  face,
			holes: []Rect2{{UMin: 5, VMin: 0, UMax: 6, VMax: 1}},
		},
		{
			name:  "hole extending past the face",
			face:  face,
			holes: []Rect2{{UMin: 3, VMin: 0, UMax: 6, VMax: 1}},
		},
		{
			name:  "overlapping holes",
			face:  face,
			holes: []Rect2{{UMin: 1, VMin: 0, UMax: 3, VMax: 2}, {UMin: 2, VMin: 0, UMax: 3.5, VMax: 2}},
		},
		{
			name:  "degenerate hole",
			face:  face,
			holes: []Rect2{{UMin: 1, VMin: 1, UMax: 1, VMax: 2}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := CutRectangularHoles(frame, test.face, test.holes, faceCutEps)
			if err == nil {
				t.Fatal("CutRectangularHoles succeeded, want an error")
			}
		})
	}
}

// A hole only slightly wider than eps is still a legal hole, and the cell
// covering it must be dropped like any other.
func TestCutRectangularHolesCutsHolesNarrowerThanTwiceEps(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})
	face := Rect2{UMax: 4, VMax: 3}
	hole := Rect2{UMin: 1, VMin: 1, UMax: 1 + 1.5*faceCutEps, VMax: 2}

	triangles, err := CutRectangularHoles(frame, face, []Rect2{hole}, faceCutEps)
	if err != nil {
		t.Fatalf("CutRectangularHoles: %v", err)
	}

	for _, triangle := range triangles {
		centroid := triangle.V0.Add(triangle.V1).Add(triangle.V2).Scale(1.0 / 3.0)
		if hole.ContainsPoint(frame.To2D(centroid), 0) {
			t.Fatalf("triangle %+v covers the hole", triangle)
		}
	}
}

func TestCutRectangularHolesIsEdgeManifold(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})
	face := Rect2{UMax: 4, VMax: 3}

	triangles, err := CutRectangularHoles(frame, face, []Rect2{{UMin: 1, VMin: 0, UMax: 2, VMax: 2}}, faceCutEps)
	if err != nil {
		t.Fatalf("CutRectangularHoles: %v", err)
	}

	// Grid cells share complete edges, so every interior edge is used exactly
	// twice. A merged triangulation would produce T-junctions and break this.
	type edge struct{ a, b Vec3 }

	uses := map[edge]int{}

	for _, triangle := range triangles {
		for _, pair := range [][2]Vec3{
			{triangle.V0, triangle.V1},
			{triangle.V1, triangle.V2},
			{triangle.V2, triangle.V0},
		} {
			key := edge{a: pair[0], b: pair[1]}
			if vec3Less(pair[1], pair[0]) {
				key = edge{a: pair[1], b: pair[0]}
			}

			uses[key]++
		}
	}

	for key, count := range uses {
		if count > 2 {
			t.Fatalf("edge %+v used %d times, want at most 2", key, count)
		}
	}

	// The use count alone cannot see a T-junction: a long edge meeting two
	// shorter ones yields three distinct keys, each used twice. Watertightness
	// additionally requires that no vertex lies in the interior of another edge.
	vertices := map[Vec3]struct{}{}

	for _, triangle := range triangles {
		vertices[triangle.V0] = struct{}{}
		vertices[triangle.V1] = struct{}{}
		vertices[triangle.V2] = struct{}{}
	}

	for key := range uses {
		for vertex := range vertices {
			if vertex == key.a || vertex == key.b {
				continue
			}

			if pointInSegmentInterior(vertex, key.a, key.b) {
				t.Fatalf("vertex %+v lies inside edge %+v, a T-junction", vertex, key)
			}
		}
	}
}

// pointInSegmentInterior reports whether p lies strictly between a and b. The
// grid vertices are exact sums of the cut lines, so an exact colinearity test
// with a small tolerance is enough here.
func pointInSegmentInterior(p, a, b Vec3) bool {
	const eps = 1e-12

	ab := b.Sub(a)
	ap := p.Sub(a)

	if ab.Cross(ap).Norm() > eps {
		return false
	}

	t := ap.Dot(ab) / ab.Dot(ab)

	return t > eps && t < 1-eps
}

func vec3Less(a, b Vec3) bool {
	if a.X != b.X {
		return a.X < b.X
	}

	if a.Y != b.Y {
		return a.Y < b.Y
	}

	return a.Z < b.Z
}

func TestRectangleFromCoplanarPolygon(t *testing.T) {
	t.Parallel()

	// The shipped fixture's door polygon on the x = 6 wall.
	frame := NewPlaneFrame(Vec3{X: 6}, Vec3{X: -1})
	polygon := []Vec3{
		{X: 6, Y: 1.4, Z: 0},
		{X: 6, Y: 2.6, Z: 0},
		{X: 6, Y: 2.6, Z: 2.1},
		{X: 6, Y: 1.4, Z: 2.1},
	}

	rect, ok := RectangleFromCoplanarPolygon(frame, polygon, 1e-7)
	if !ok {
		t.Fatal("RectangleFromCoplanarPolygon rejected the shipped door polygon")
	}

	if got, want := rectArea(rect), 1.2*2.1; math.Abs(got-want) > 1e-9 {
		t.Fatalf("rect area = %v, want %v", got, want)
	}

	offPlane := append([]Vec3(nil), polygon...)
	offPlane[2].X = 6.5

	if _, ok := RectangleFromCoplanarPolygon(frame, offPlane, 1e-7); ok {
		t.Fatal("RectangleFromCoplanarPolygon accepted a non-planar polygon")
	}
}
