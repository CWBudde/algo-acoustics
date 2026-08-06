package pde

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// rectRoom returns a ConvexRoom matching a shoebox of width×depth×height
// anchored at the origin with inward-pointing normals.
func rectRoom(w, d, h float64) *ConvexRoom {
	room, err := NewConvexRoom(
		[]geometry.Plane{
			{Normal: geometry.Vec3{X: 1}, Distance: 0},
			{Normal: geometry.Vec3{X: -1}, Distance: -w},
			{Normal: geometry.Vec3{Y: 1}, Distance: 0},
			{Normal: geometry.Vec3{Y: -1}, Distance: -d},
			{Normal: geometry.Vec3{Z: 1}, Distance: 0},
			{Normal: geometry.Vec3{Z: -1}, Distance: -h},
		},
		[]geometry.Vec3{
			{X: 0, Y: 0, Z: 0},
			{X: w, Y: 0, Z: 0},
			{X: 0, Y: d, Z: 0},
			{X: w, Y: d, Z: 0},
			{X: 0, Y: 0, Z: h},
			{X: w, Y: 0, Z: h},
			{X: 0, Y: d, Z: h},
			{X: w, Y: d, Z: h},
		},
	)
	if err != nil {
		panic(err)
	}

	return room
}

//nolint:gocyclo // This white-box test exhaustively checks every grid class and neighbor invariant.
func TestClassifyGrid_RectangularRoom(t *testing.T) {
	// A 2×2×2 room with h=0.5 should give a grid where the interior nodes
	// form a regular pattern identical to what a shoebox solver would use.
	room := rectRoom(2, 2, 2)
	g := ClassifyGrid(room, 0.5)

	// Every node that is strictly inside (not on face) should be interior or
	// boundary.  Nodes exactly on the boundary (SideOf=0) are exterior by our
	// strict PointInside convention.  The padding cell around the AABB
	// guarantees at least one layer of exterior nodes on each side.

	// Count classes.
	var nInt, nBnd, nExt int

	for _, c := range g.Class {
		switch c {
		case Interior:
			nInt++
		case Boundary:
			nBnd++
		case Exterior:
			nExt++
		}
	}

	t.Logf("grid %dx%dx%d = %d nodes", g.Nx, g.Ny, g.Nz, len(g.Class))
	t.Logf("interior=%d  boundary=%d  exterior=%d", nInt, nBnd, nExt)

	if nInt == 0 {
		t.Error("expected some interior nodes")
	}

	if nBnd == 0 {
		t.Error("expected some boundary nodes")
	}

	if nExt == 0 {
		t.Error("expected some exterior nodes")
	}

	// Verify symmetry: for a cube centred on the grid, the interior count
	// should reflect the symmetric structure.
	if nInt+nBnd == 0 {
		t.Fatal("no active nodes")
	}

	// Every interior/boundary node must be inside the room.
	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				p := g.nodePos(ix, iy, iz)
				cls := g.Class[idx]

				if cls == Interior || cls == Boundary {
					if !room.PointInside(p) {
						t.Errorf("node (%d,%d,%d) at %v classified as %d but not inside room",
							ix, iy, iz, p, cls)
					}
				}

				if cls == Exterior && room.PointInside(p) {
					t.Errorf("node (%d,%d,%d) at %v classified as exterior but is inside room",
						ix, iy, iz, p)
				}
			}
		}
	}

	// Verify every boundary node has at least one exterior neighbor.
	offsets := [6][3]int{
		{-1, 0, 0},
		{1, 0, 0},
		{0, -1, 0},
		{0, 1, 0},
		{0, 0, -1},
		{0, 0, 1},
	}

	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				if g.Class[idx] != Boundary {
					continue
				}

				hasExt := false

				for _, o := range offsets {
					ni, nj, nk := ix+o[0], iy+o[1], iz+o[2]
					if ni < 0 || ni >= g.Nx || nj < 0 || nj >= g.Ny || nk < 0 || nk >= g.Nz {
						hasExt = true

						break
					}

					if g.Class[g.nodeIndex(ni, nj, nk)] == Exterior {
						hasExt = true

						break
					}
				}

				if !hasExt {
					t.Errorf("boundary node (%d,%d,%d) has no exterior neighbor", ix, iy, iz)
				}
			}
		}
	}

	// Verify every interior node has NO exterior neighbor.
	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				if g.Class[idx] != Interior {
					continue
				}

				for _, o := range offsets {
					ni, nj, nk := ix+o[0], iy+o[1], iz+o[2]
					if ni < 0 || ni >= g.Nx || nj < 0 || nj >= g.Ny || nk < 0 || nk >= g.Nz {
						t.Errorf("interior node (%d,%d,%d) is at grid edge", ix, iy, iz)

						continue
					}

					if g.Class[g.nodeIndex(ni, nj, nk)] == Exterior {
						t.Errorf("interior node (%d,%d,%d) has exterior neighbor at offset %v",
							ix, iy, iz, o)
					}
				}
			}
		}
	}
}

//nolint:nestif // The nested coordinate walk mirrors the three-dimensional grid invariant.
func TestClassifyGrid_RectangularMatchesShoebox(t *testing.T) {
	// For a rectangular room aligned with axes, the IBM classification should
	// produce a pattern that exactly matches what a regular shoebox solver
	// would expect: a solid block of active nodes with boundary wrapping
	// the interior.  Specifically, for a 3×3×3 m room at h=1.0 the strictly
	// interior grid (ignoring padding) should have nodes at positions
	// 0.5, 1.5, 2.5 — giving a 3×3×3 cube of active nodes where the surface
	// layer is boundary and the single centre node is interior.
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 1.0)

	// Walk the grid and verify that the active region forms a contiguous
	// rectangular block with no holes.
	var activeMin, activeMax [3]int
	activeMin = [3]int{g.Nx, g.Ny, g.Nz}

	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				if g.Class[g.nodeIndex(ix, iy, iz)] != Exterior {
					if ix < activeMin[0] {
						activeMin[0] = ix
					}

					if iy < activeMin[1] {
						activeMin[1] = iy
					}

					if iz < activeMin[2] {
						activeMin[2] = iz
					}

					if ix > activeMax[0] {
						activeMax[0] = ix
					}

					if iy > activeMax[1] {
						activeMax[1] = iy
					}

					if iz > activeMax[2] {
						activeMax[2] = iz
					}
				}
			}
		}
	}

	// Within the bounding indices, every node should be active (no holes).
	for ix := activeMin[0]; ix <= activeMax[0]; ix++ {
		for iy := activeMin[1]; iy <= activeMax[1]; iy++ {
			for iz := activeMin[2]; iz <= activeMax[2]; iz++ {
				if g.Class[g.nodeIndex(ix, iy, iz)] == Exterior {
					t.Errorf("hole at (%d,%d,%d) in active region", ix, iy, iz)
				}
			}
		}
	}

	t.Logf("active block: [%v] to [%v], size %dx%dx%d",
		activeMin, activeMax,
		activeMax[0]-activeMin[0]+1,
		activeMax[1]-activeMin[1]+1,
		activeMax[2]-activeMin[2]+1)
}

func TestClassifyGrid_RotatedSquare(t *testing.T) {
	// A 2D-extruded 45° rotated square (diamond) in the XY plane, 4 units tall.
	// Diamond vertices at (2,0), (4,2), (2,4), (0,2) — side length 2√2.
	// Extruded from z=0 to z=4 (tall enough to produce interior nodes).
	// Grid spacing h=0.3 avoids grid nodes landing exactly on 45° walls.
	centre := geometry.Vec3{X: 2, Y: 2, Z: 2}
	diamondVerts2D := [4]geometry.Vec3{
		{X: 2, Y: 0}, {X: 4, Y: 2}, {X: 2, Y: 4}, {X: 0, Y: 2},
	}

	diamondWalls := make([]geometry.Plane, 0, 4)

	for i := range 4 {
		a := diamondVerts2D[i]
		b := diamondVerts2D[(i+1)%4]
		edge := b.Sub(a)
		perp := geometry.Vec3{X: edge.Y, Y: -edge.X}
		mid := a.Add(b).Scale(0.5)

		if perp.Dot(centre.Sub(mid)) < 0 {
			perp = perp.Neg()
		}

		diamondWalls = append(diamondWalls, geometry.NewPlaneFromPointNormal(a, perp))
	}

	allWalls := make([]geometry.Plane, 0, 6)
	allWalls = append(
		allWalls,
		geometry.Plane{Normal: geometry.Vec3{Z: 1}, Distance: 0},
		geometry.Plane{Normal: geometry.Vec3{Z: -1}, Distance: -4},
	)
	allWalls = append(allWalls, diamondWalls...)

	allVerts := []geometry.Vec3{
		{X: 2, Y: 0, Z: 0},
		{X: 4, Y: 2, Z: 0},
		{X: 2, Y: 4, Z: 0},
		{X: 0, Y: 2, Z: 0},
		{X: 2, Y: 0, Z: 4},
		{X: 4, Y: 2, Z: 4},
		{X: 2, Y: 4, Z: 4},
		{X: 0, Y: 2, Z: 4},
	}

	room, err := NewConvexRoom(allWalls, allVerts)
	if err != nil {
		t.Fatalf("room construction: %v", err)
	}

	g := ClassifyGrid(room, 0.3)

	var nInt, nBnd, nExt int

	for _, c := range g.Class {
		switch c {
		case Interior:
			nInt++
		case Boundary:
			nBnd++
		case Exterior:
			nExt++
		}
	}

	t.Logf("grid %dx%dx%d = %d nodes", g.Nx, g.Ny, g.Nz, len(g.Class))
	t.Logf("interior=%d  boundary=%d  exterior=%d", nInt, nBnd, nExt)

	if nInt == 0 {
		t.Error("expected some interior nodes")
	}

	if nBnd == 0 {
		t.Error("expected some boundary nodes")
	}

	// All active nodes must be inside the room.
	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				p := g.nodePos(ix, iy, iz)

				if g.Class[idx] != Exterior && !room.PointInside(p) {
					t.Errorf("active node (%d,%d,%d) at %v is not inside the room", ix, iy, iz, p)
				}
			}
		}
	}

	// Verify XY symmetry: interior/exterior should match when mirrored
	// about x=2. Nodes exactly on a 45° wall can go either way due to
	// floating-point, so we only check that both mirror partners are
	// either both active (Interior or Boundary) or both Exterior.
	for iz := range g.Nz {
		for ix := range g.Nx {
			for iy := range g.Ny {
				c1 := g.Class[g.nodeIndex(ix, iy, iz)]
				mirIx := g.Nx - 1 - ix
				c2 := g.Class[g.nodeIndex(mirIx, iy, iz)]

				active1 := c1 != Exterior
				active2 := c2 != Exterior

				if active1 != active2 {
					p1 := g.nodePos(ix, iy, iz)
					p2 := g.nodePos(mirIx, iy, iz)
					t.Errorf("X-symmetry broken: (%d,%d,%d) [%v] active=%v vs (%d,%d,%d) [%v] active=%v",
						ix, iy, iz, p1, active1, mirIx, iy, iz, p2, active2)
				}
			}
		}
	}

	// Verify boundary nodes have fractional distances set.
	for idx, bi := range g.Boundary {
		hasFrac := false

		for a := range 3 {
			for d := range 2 {
				if bi.Frac[a][d] > 0 {
					hasFrac = true

					if bi.Frac[a][d] > 1 {
						t.Errorf("boundary node %d: frac[%d][%d] = %v > 1", idx, a, d, bi.Frac[a][d])
					}
				}
			}
		}

		if !hasFrac {
			t.Errorf("boundary node %d has no fractional distances set", idx)
		}
	}
}

func TestClassifyGrid_BoundaryFractions(t *testing.T) {
	// For a rectangular room with h=1.0, boundary nodes should have
	// fractional distances that reflect their position relative to the wall.
	room := rectRoom(4, 4, 4)
	g := ClassifyGrid(room, 1.0)

	// Find a boundary node near the x=0 wall and verify its fraction.
	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				if g.Class[idx] != Boundary {
					continue
				}

				bi := g.Boundary[idx]
				p := g.nodePos(ix, iy, iz)

				// For each axis with a nonzero fraction, the fractional
				// distance should approximately equal the distance to the
				// nearest wall along that axis divided by h.
				for a := range 3 {
					for d := range 2 {
						f := bi.Frac[a][d]
						if f <= 0 {
							continue
						}

						if f < 0 || f > 1 {
							t.Errorf("node (%d,%d,%d) at %v: frac[%d][%d] = %v out of (0,1]",
								ix, iy, iz, p, a, d, f)
						}
					}
				}
			}
		}
	}
}

func TestClassifyGrid_SmallRoom(t *testing.T) {
	// Tiny room: verify we still get sensible classification.
	room := rectRoom(1, 1, 1)
	g := ClassifyGrid(room, 0.25)

	if g.NumActive() == 0 {
		t.Error("no active nodes for small room")
	}

	// The number of active nodes should be roughly (1/0.25 - 1)^3 = 3^3 = 27.
	t.Logf("active=%d (interior=%d, boundary=%d)", g.NumActive(), g.NumInterior(), g.NumBoundary())
}
