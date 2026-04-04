package pde

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// --- helper constructors for test rooms ---

// cubeRoom returns a 2×2×2 cube centred at (1,1,1) with inward normals.
func cubeRoom() (*ConvexRoom, error) {
	walls := []geometry.Plane{
		{Normal: geometry.Vec3{X: 1}, Distance: 0},   // x = 0, normal +x
		{Normal: geometry.Vec3{X: -1}, Distance: -2}, // x = 2, normal −x
		{Normal: geometry.Vec3{Y: 1}, Distance: 0},   // y = 0
		{Normal: geometry.Vec3{Y: -1}, Distance: -2}, // y = 2
		{Normal: geometry.Vec3{Z: 1}, Distance: 0},   // z = 0
		{Normal: geometry.Vec3{Z: -1}, Distance: -2}, // z = 2
	}
	verts := []geometry.Vec3{
		{X: 0, Y: 0, Z: 0},
		{X: 2, Y: 0, Z: 0},
		{X: 0, Y: 2, Z: 0},
		{X: 2, Y: 2, Z: 0},
		{X: 0, Y: 0, Z: 2},
		{X: 2, Y: 0, Z: 2},
		{X: 0, Y: 2, Z: 2},
		{X: 2, Y: 2, Z: 2},
	}

	return NewConvexRoom(walls, verts)
}

// wedgeRoom returns a triangular-prism room (5 walls):
//
//	base at y=0, top at y=3, triangular cross-section in XZ:
//	vertices (0,0,0)-(4,0,0)-(2,0,3) extruded along Y to y=3.
func wedgeRoom() (*ConvexRoom, error) {
	// bottom face: y=0, normal +Y
	// top face:    y=3, normal −Y
	// back wall:   z=0, normal +Z
	// left slope:  from (0,0,0) to (2,0,3) → outward normal (−3,0,2)/√13
	//              inward normal = (3,0,−2)/√13 ... wait, need to think about this.
	//
	// The triangle in XZ is (0,0)-(4,0)-(2,3).
	// Edge (0,0)→(2,3): direction (2,3). Outward normal points left: (−3,2)/√13.
	//   Inward normal: (3,−2)/√13. Plane passes through (0,0,0).
	// Edge (4,0)→(2,3): direction (−2,3). Outward normal points right: (3,2)/√13.
	//   Inward normal: (−3,−2)/√13. Plane passes through (4,0,0).
	// Edge (0,0)→(4,0): z=0, inward normal +Z. Already covered.

	s := 1.0 / math.Sqrt(13)

	walls := []geometry.Plane{
		{Normal: geometry.Vec3{Y: 1}, Distance: 0},   // y = 0
		{Normal: geometry.Vec3{Y: -1}, Distance: -3}, // y = 3
		{Normal: geometry.Vec3{Z: 1}, Distance: 0},   // z = 0 (base edge)
		// left slope: through (0,_,0), inward normal (3,0,-2)/√13
		{Normal: geometry.Vec3{X: 3 * s, Z: -2 * s}, Distance: 0},
		// right slope: through (4,_,0), inward normal (-3,0,-2)/√13
		// d = n·(4,0,0) = -3*4/√13 = -12/√13
		{Normal: geometry.Vec3{X: -3 * s, Z: -2 * s}, Distance: -12 * s},
	}

	verts := []geometry.Vec3{
		{X: 0, Y: 0, Z: 0},
		{X: 4, Y: 0, Z: 0},
		{X: 2, Y: 0, Z: 3},
		{X: 0, Y: 3, Z: 0},
		{X: 4, Y: 3, Z: 0},
		{X: 2, Y: 3, Z: 3},
	}

	return NewConvexRoom(walls, verts)
}

// truncatedPyramidRoom returns a frustum (truncated pyramid) with 6 walls:
// bottom square 4×4 at z=0, top square 2×2 at z=3, centred on x=2, y=2.
func truncatedPyramidRoom() (*ConvexRoom, error) {
	// The four sloped walls connect bottom edges to top edges.
	// Bottom: (0,0,0)-(4,0,0)-(4,4,0)-(0,4,0)
	// Top:    (1,1,3)-(3,1,3)-(3,3,3)-(1,3,3)
	//
	// Front wall (low-Y side): passes through (0,0,0),(4,0,0),(3,1,3),(1,1,3)
	//   Two edge vectors: (4,0,0) and (1,1,3). Normal = cross = (0·3−0·1, 0·1−4·3, 4·1−0·1) = (0,−12,4).
	//   Inward (toward +Y): (0,12,4). Normalise: len=√(144+16)=√160=4√10.
	//   n = (0, 3/√10, 1/√10). d = n·(0,0,0) = 0.

	s10 := 1.0 / math.Sqrt(10)

	walls := []geometry.Plane{
		// bottom z=0, inward +Z
		{Normal: geometry.Vec3{Z: 1}, Distance: 0},
		// top z=3, inward −Z
		{Normal: geometry.Vec3{Z: -1}, Distance: -3},
		// front (y=0 side), inward normal (0, 3, 1)/√10
		{Normal: geometry.Vec3{Y: 3 * s10, Z: s10}, Distance: 0},
		// back (y=4 side), inward normal (0, −3, 1)/√10, through (0,4,0)
		// d = n·(0,4,0) = −12/√10
		{Normal: geometry.Vec3{Y: -3 * s10, Z: s10}, Distance: -12 * s10},
		// left (x=0 side), inward normal (3, 0, 1)/√10
		{Normal: geometry.Vec3{X: 3 * s10, Z: s10}, Distance: 0},
		// right (x=4 side), inward normal (−3, 0, 1)/√10, through (4,0,0)
		// d = n·(4,0,0) = −12/√10
		{Normal: geometry.Vec3{X: -3 * s10, Z: s10}, Distance: -12 * s10},
	}

	verts := []geometry.Vec3{
		// bottom
		{X: 0, Y: 0, Z: 0},
		{X: 4, Y: 0, Z: 0},
		{X: 4, Y: 4, Z: 0},
		{X: 0, Y: 4, Z: 0},
		// top
		{X: 1, Y: 1, Z: 3},
		{X: 3, Y: 1, Z: 3},
		{X: 3, Y: 3, Z: 3},
		{X: 1, Y: 3, Z: 3},
	}

	return NewConvexRoom(walls, verts)
}

// --- Tests ---

func TestConvexRoom_Construction(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (*ConvexRoom, error)
	}{
		{"cube", cubeRoom},
		{"wedge", wedgeRoom},
		{"truncated_pyramid", truncatedPyramidRoom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := tt.fn()
			if err != nil {
				t.Fatalf("construction failed: %v", err)
			}
			if r == nil {
				t.Fatal("room is nil")
			}
		})
	}
}

func TestConvexRoom_RejectsTooFewWalls(t *testing.T) {
	walls := []geometry.Plane{
		{Normal: geometry.Vec3{X: 1}, Distance: 0},
		{Normal: geometry.Vec3{X: -1}, Distance: -1},
		{Normal: geometry.Vec3{Y: 1}, Distance: 0},
	}
	verts := []geometry.Vec3{{}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}

	_, err := NewConvexRoom(walls, verts)
	if err == nil {
		t.Fatal("expected error for < 4 walls")
	}
}

func TestConvexRoom_RejectsNonConvex(t *testing.T) {
	// Cube walls but add a vertex clearly outside.
	walls := []geometry.Plane{
		{Normal: geometry.Vec3{X: 1}, Distance: 0},
		{Normal: geometry.Vec3{X: -1}, Distance: -2},
		{Normal: geometry.Vec3{Y: 1}, Distance: 0},
		{Normal: geometry.Vec3{Y: -1}, Distance: -2},
		{Normal: geometry.Vec3{Z: 1}, Distance: 0},
		{Normal: geometry.Vec3{Z: -1}, Distance: -2},
	}
	verts := []geometry.Vec3{
		{X: 0, Y: 0, Z: 0},
		{X: 2, Y: 0, Z: 0},
		{X: 0, Y: 2, Z: 0},
		{X: 5, Y: 5, Z: 5}, // outside
	}

	_, err := NewConvexRoom(walls, verts)
	if err == nil {
		t.Fatal("expected error for vertex outside walls")
	}
}

func TestConvexRoom_PointInside(t *testing.T) {
	type pointTest struct {
		p    geometry.Vec3
		want bool
	}

	tests := []struct {
		name   string
		fn     func() (*ConvexRoom, error)
		points []pointTest
	}{
		{
			name: "cube",
			fn:   cubeRoom,
			points: []pointTest{
				{geometry.Vec3{X: 1, Y: 1, Z: 1}, true},       // centre
				{geometry.Vec3{X: 0.1, Y: 0.1, Z: 0.1}, true}, // near corner
				{geometry.Vec3{X: 1.9, Y: 1.9, Z: 1.9}, true}, // near opposite corner
				{geometry.Vec3{X: 0, Y: 1, Z: 1}, false},      // on face (not strictly inside)
				{geometry.Vec3{X: -0.1, Y: 1, Z: 1}, false},   // outside
				{geometry.Vec3{X: 3, Y: 3, Z: 3}, false},      // far outside
			},
		},
		{
			name: "wedge",
			fn:   wedgeRoom,
			points: []pointTest{
				{geometry.Vec3{X: 2, Y: 1.5, Z: 1}, true},     // centre-ish
				{geometry.Vec3{X: 0.5, Y: 0.5, Z: 0.1}, true}, // near base corner
				{geometry.Vec3{X: 2, Y: 1.5, Z: 2.5}, true},   // near apex edge
				{geometry.Vec3{X: 2, Y: 1.5, Z: 3.5}, false},  // above apex
				{geometry.Vec3{X: -1, Y: 1.5, Z: 1}, false},   // outside left
			},
		},
		{
			name: "truncated_pyramid",
			fn:   truncatedPyramidRoom,
			points: []pointTest{
				{geometry.Vec3{X: 2, Y: 2, Z: 1.5}, true},     // centre
				{geometry.Vec3{X: 0.5, Y: 0.5, Z: 0.1}, true}, // near bottom corner
				{geometry.Vec3{X: 1.5, Y: 1.5, Z: 2.5}, true}, // near top face
				{geometry.Vec3{X: 0, Y: 0, Z: 3}, false},      // outside top corner
				{geometry.Vec3{X: 2, Y: 2, Z: -1}, false},     // below floor
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := tt.fn()
			if err != nil {
				t.Fatalf("room: %v", err)
			}

			for _, pt := range tt.points {
				got := r.PointInside(pt.p)
				if got != pt.want {
					t.Errorf("PointInside(%v) = %v, want %v", pt.p, got, pt.want)
				}
			}
		})
	}
}

func TestConvexRoom_DistanceToNearestWall(t *testing.T) {
	r, err := cubeRoom()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		p        geometry.Vec3
		wantDist float64
		wantTol  float64
	}{
		{"centre", geometry.Vec3{X: 1, Y: 1, Z: 1}, 1.0, 1e-12},
		{"near_xlow", geometry.Vec3{X: 0.2, Y: 1, Z: 1}, 0.2, 1e-12},
		{"near_corner", geometry.Vec3{X: 0.1, Y: 0.1, Z: 0.1}, 0.1, 1e-12},
		{"on_face", geometry.Vec3{X: 0, Y: 1, Z: 1}, 0.0, 1e-12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := r.DistanceToNearestWall(tt.p)
			if math.Abs(wd.Dist-tt.wantDist) > tt.wantTol {
				t.Errorf("dist = %v, want %v (±%v)", wd.Dist, tt.wantDist, tt.wantTol)
			}
			if wd.WallIdx < 0 || wd.WallIdx >= len(r.Walls) {
				t.Errorf("wallIdx = %d, out of range", wd.WallIdx)
			}
		})
	}
}

func TestConvexRoom_DistanceToNearestWall_Wedge(t *testing.T) {
	r, err := wedgeRoom()
	if err != nil {
		t.Fatal(err)
	}

	// A point near the base (z=0) should have z-distance as nearest.
	wd := r.DistanceToNearestWall(geometry.Vec3{X: 2, Y: 1.5, Z: 0.05})
	if wd.Dist < 0 {
		t.Errorf("interior point has negative distance: %v", wd.Dist)
	}

	if math.Abs(wd.Dist-0.05) > 0.01 {
		t.Errorf("expected ~0.05, got %v", wd.Dist)
	}
}

func TestConvexRoom_BoundingBox(t *testing.T) {
	tests := []struct {
		name    string
		fn      func() (*ConvexRoom, error)
		padding float64
		wantMin geometry.Vec3
		wantMax geometry.Vec3
	}{
		{
			name:    "cube_no_padding",
			fn:      cubeRoom,
			padding: 0,
			wantMin: geometry.Vec3{X: 0, Y: 0, Z: 0},
			wantMax: geometry.Vec3{X: 2, Y: 2, Z: 2},
		},
		{
			name:    "cube_with_padding",
			fn:      cubeRoom,
			padding: 0.5,
			wantMin: geometry.Vec3{X: -0.5, Y: -0.5, Z: -0.5},
			wantMax: geometry.Vec3{X: 2.5, Y: 2.5, Z: 2.5},
		},
		{
			name:    "wedge_no_padding",
			fn:      wedgeRoom,
			padding: 0,
			wantMin: geometry.Vec3{X: 0, Y: 0, Z: 0},
			wantMax: geometry.Vec3{X: 4, Y: 3, Z: 3},
		},
		{
			name:    "truncated_pyramid_no_padding",
			fn:      truncatedPyramidRoom,
			padding: 0,
			wantMin: geometry.Vec3{X: 0, Y: 0, Z: 0},
			wantMax: geometry.Vec3{X: 4, Y: 4, Z: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := tt.fn()
			if err != nil {
				t.Fatal(err)
			}

			bb := r.BoundingBox(tt.padding)
			if !vec3Near(bb.Min, tt.wantMin, 1e-12) {
				t.Errorf("Min = %v, want %v", bb.Min, tt.wantMin)
			}
			if !vec3Near(bb.Max, tt.wantMax, 1e-12) {
				t.Errorf("Max = %v, want %v", bb.Max, tt.wantMax)
			}
		})
	}
}

func vec3Near(a, b geometry.Vec3, tol float64) bool {
	return math.Abs(a.X-b.X) < tol &&
		math.Abs(a.Y-b.Y) < tol &&
		math.Abs(a.Z-b.Z) < tol
}
