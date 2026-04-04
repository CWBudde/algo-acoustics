package pde

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// NodeClass describes whether a grid node is inside, on the boundary of,
// or outside the convex room.
type NodeClass uint8

const (
	// Exterior is a node outside the room (inactive, pressure = 0).
	Exterior NodeClass = iota
	// Interior is a node fully inside the room with all 6 neighbors also inside.
	Interior
	// Boundary is a node inside the room with at least one exterior neighbor.
	Boundary
)

// BoundaryInfo stores sub-cell geometry for a single boundary node.
// Frac[axis][dir] is the fractional distance (in 0,1] from this node to
// the nearest wall along the given axis and direction.  axis: 0=X, 1=Y, 2=Z;
// dir: 0=negative, 1=positive.  A value of 0 means the neighbor in that
// direction is interior (no wall crossing within one cell).
type BoundaryInfo struct {
	Frac    [3][2]float64 // [axis][dir]
	Normal  geometry.Vec3 // inward normal of nearest wall
	WallIdx int           // index into ConvexRoom.Walls
}

// IBMGrid is an immersed-boundary grid that classifies regular Cartesian
// nodes relative to a convex room geometry.
type IBMGrid struct {
	Nx, Ny, Nz int           // number of nodes per axis
	H          float64       // uniform grid spacing (metres)
	Origin     geometry.Vec3 // world position of node (0,0,0)

	Class    []NodeClass          // flat array [Nx*Ny*Nz], row-major
	Boundary map[int]BoundaryInfo // keyed by flat index, only for Boundary nodes
}

// nodeIndex returns the flat index for grid coordinates (ix,iy,iz).
func (g *IBMGrid) nodeIndex(ix, iy, iz int) int {
	return ix*g.Ny*g.Nz + iy*g.Nz + iz
}

// nodePos returns the world position of grid node (ix,iy,iz).
func (g *IBMGrid) nodePos(ix, iy, iz int) geometry.Vec3 {
	return geometry.Vec3{
		X: g.Origin.X + float64(ix)*g.H,
		Y: g.Origin.Y + float64(iy)*g.H,
		Z: g.Origin.Z + float64(iz)*g.H,
	}
}

// NumInterior returns the number of interior nodes.
func (g *IBMGrid) NumInterior() int {
	n := 0
	for _, c := range g.Class {
		if c == Interior {
			n++
		}
	}

	return n
}

// NumBoundary returns the number of boundary nodes.
func (g *IBMGrid) NumBoundary() int { return len(g.Boundary) }

// NumActive returns interior + boundary (nodes that participate in the solve).
func (g *IBMGrid) NumActive() int { return g.NumInterior() + g.NumBoundary() }

// ClassifyGrid builds an IBMGrid for the given convex room on a uniform
// Cartesian grid with spacing h.  The grid covers the room's bounding box
// plus one cell of padding on each side (so boundary nodes always have
// valid neighbor lookups).
func ClassifyGrid(room *ConvexRoom, h float64) *IBMGrid {
	bb := room.BoundingBox(h) // one cell of padding
	center := bb.Center()
	dims := bb.Dimensions()

	// Use odd grid counts so the centre of symmetry falls on a node.
	halfNx := int(math.Ceil(dims.X / (2 * h)))
	halfNy := int(math.Ceil(dims.Y / (2 * h)))
	halfNz := int(math.Ceil(dims.Z / (2 * h)))

	nx := 2*halfNx + 1
	ny := 2*halfNy + 1
	nz := 2*halfNz + 1

	origin := geometry.Vec3{
		X: center.X - float64(halfNx)*h,
		Y: center.Y - float64(halfNy)*h,
		Z: center.Z - float64(halfNz)*h,
	}

	g := &IBMGrid{
		Nx:       nx,
		Ny:       ny,
		Nz:       nz,
		H:        h,
		Origin:   origin,
		Class:    make([]NodeClass, nx*ny*nz),
		Boundary: make(map[int]BoundaryInfo),
	}

	// Pass 1: mark every node as interior or exterior.
	inside := make([]bool, nx*ny*nz)
	for ix := range nx {
		for iy := range ny {
			for iz := range nz {
				p := g.nodePos(ix, iy, iz)
				idx := g.nodeIndex(ix, iy, iz)
				if room.PointInside(p) {
					inside[idx] = true
					g.Class[idx] = Interior
				}
				// Exterior is zero-value, already set.
			}
		}
	}

	// Pass 2: interior nodes with at least one exterior 6-connected neighbor
	// become boundary nodes.
	offsets := [6][3]int{
		{-1, 0, 0},
		{1, 0, 0},
		{0, -1, 0},
		{0, 1, 0},
		{0, 0, -1},
		{0, 0, 1},
	}

	for ix := range nx {
		for iy := range ny {
			for iz := range nz {
				idx := g.nodeIndex(ix, iy, iz)
				if !inside[idx] {
					continue
				}

				hasExterior := false
				for _, o := range offsets {
					ni, nj, nk := ix+o[0], iy+o[1], iz+o[2]
					if ni < 0 || ni >= nx || nj < 0 || nj >= ny || nk < 0 || nk >= nz {
						hasExterior = true

						break
					}

					if !inside[g.nodeIndex(ni, nj, nk)] {
						hasExterior = true

						break
					}
				}

				if !hasExterior {
					continue
				}

				g.Class[idx] = Boundary

				// Compute boundary info.
				p := g.nodePos(ix, iy, iz)
				wd := room.DistanceToNearestWall(p)

				bi := BoundaryInfo{
					Normal:  wd.Normal,
					WallIdx: wd.WallIdx,
				}

				// For each axis direction, find fractional wall distance if
				// the neighbor is exterior.
				axes := [3]geometry.Vec3{
					{X: 1}, {Y: 1}, {Z: 1},
				}

				for a := range 3 {
					for d := range 2 {
						dir := axes[a]
						if d == 0 {
							dir = dir.Neg()
						}

						// Check if neighbor in this direction is exterior.
						ni := ix + offsets[a*2+d][0]
						nj := iy + offsets[a*2+d][1]
						nk := iz + offsets[a*2+d][2]

						if ni < 0 || ni >= nx || nj < 0 || nj >= ny || nk < 0 || nk >= nz || !inside[g.nodeIndex(ni, nj, nk)] {
							// Find nearest wall intersection along this direction.
							frac := fracToWall(room, p, dir, h)
							bi.Frac[a][d] = frac
						}
					}
				}

				g.Boundary[idx] = bi
			}
		}
	}

	return g
}

// fracToWall finds the fractional distance (in 0,1] from point p along
// direction dir to the nearest wall of the convex room, normalised by h.
// Returns 0 if no wall is hit within one cell.
func fracToWall(room *ConvexRoom, p, dir geometry.Vec3, h float64) float64 {
	bestT := math.Inf(1)

	for _, w := range room.Walls {
		denom := w.Normal.Dot(dir)
		if denom >= 0 {
			// Ray moves away from or parallel to this wall's interior side.
			continue
		}

		// t = (Distance - Normal·p) / (Normal·dir)
		t := (w.Distance - w.Normal.Dot(p)) / denom
		if t > 0 && t < bestT {
			bestT = t
		}
	}

	if math.IsInf(bestT, 1) {
		return 0
	}

	frac := bestT / h
	if frac > 1 {
		return 0 // wall is beyond the next grid node
	}

	return frac
}
