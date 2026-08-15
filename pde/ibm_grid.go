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
// Frac[axis][dir] is the fractional distance from this node to the nearest
// wall along the given axis and direction, normalised by the grid spacing.
// axis: 0=X, 1=Y, 2=Z; dir: 0=negative, 1=positive.
//
// Exactly two cases are representable, and they are mutually exclusive:
//
//	Frac == 0        the neighbour in that direction is an active node
//	                 (Interior or Boundary), so no wall is crossed and the
//	                 stencil reads that neighbour at the full distance h.
//	Frac in (0, 1]   the neighbour in that direction is exterior, so a wall
//	                 lies between the two, at distance Frac*h.  The stencil
//	                 uses the ghost value from the wall boundary condition.
//
// A direction with an exterior neighbour therefore never carries Frac == 0.
// That combination used to be reachable (see fracToWall) and silently imposed
// a pressure-release wall regardless of the configured WallBC, because the
// stencil fell through to reading the exterior neighbour's pressure, which is
// pinned at zero.  Keeping the two cases disjoint is what makes the classifier
// independent of whether a wall plane rounds just above or just below a node
// plane — see docs/maintenance.md on FMA contraction.
type BoundaryInfo struct {
	Frac    [3][2]float64 // [axis][dir]
	Normal  geometry.Vec3 // inward normal of nearest wall
	WallIdx int           // index into ConvexRoom.Walls
}

// boundaryNodeRef stores a boundary node's flat index, grid coordinates,
// and geometry info for efficient sparse iteration without map lookups.
type boundaryNodeRef struct {
	Idx        int
	Ix, Iy, Iz int
	Info       BoundaryInfo
}

// IBMGrid is an immersed-boundary grid that classifies regular Cartesian
// nodes relative to a convex room geometry.
type IBMGrid struct {
	Nx, Ny, Nz int           // number of nodes per axis
	H          float64       // uniform grid spacing (metres)
	Origin     geometry.Vec3 // world position of node (0,0,0)

	Class    []NodeClass          // flat array [Nx*Ny*Nz], row-major
	Boundary map[int]BoundaryInfo // keyed by flat index, only for Boundary nodes

	// Pre-computed active node lists for sparse iteration.
	// These skip exterior nodes entirely, avoiding wasted compute on
	// bounding-box padding.
	InteriorIdx []int             // flat indices of Interior nodes
	BoundaryIdx []boundaryNodeRef // flat index + grid coords for Boundary nodes

	// Compressed storage for low-fill-ratio grids.
	// When Compressed is true, pressure fields are sized NumActive()
	// instead of Nx*Ny*Nz. CompactMap translates flat grid indices to
	// compact indices (-1 for exterior nodes).
	Compressed bool
	CompactMap []int // flat index → compact index; -1 = exterior
}

// NumInterior returns the number of interior nodes.
func (g *IBMGrid) NumInterior() int { return len(g.InteriorIdx) }

// NumBoundary returns the number of boundary nodes.
func (g *IBMGrid) NumBoundary() int { return len(g.Boundary) }

// NumActive returns interior + boundary (nodes that participate in the solve).
func (g *IBMGrid) NumActive() int { return g.NumInterior() + g.NumBoundary() }

// FillRatio returns the fraction of grid nodes that are active (interior + boundary).
func (g *IBMGrid) FillRatio() float64 {
	total := g.Nx * g.Ny * g.Nz
	if total == 0 {
		return 0
	}

	return float64(g.NumActive()) / float64(total)
}

// EnableCompression switches the grid to compressed storage mode.
// Pressure fields allocated via NewField will be sized NumActive()
// instead of Nx*Ny*Nz. The stencil automatically adapts.
func (g *IBMGrid) EnableCompression() {
	if g.Compressed {
		return
	}

	total := g.Nx * g.Ny * g.Nz
	g.CompactMap = make([]int, total)

	for i := range total {
		g.CompactMap[i] = -1
	}

	ci := 0

	for _, idx := range g.InteriorIdx {
		g.CompactMap[idx] = ci
		ci++
	}

	for _, ref := range g.BoundaryIdx {
		g.CompactMap[ref.Idx] = ci
		ci++
	}

	g.Compressed = true
}

// NewField allocates a zero-initialized pressure field for this grid.
// Returns a compact field if compression is enabled, otherwise a full grid.
func (g *IBMGrid) NewField() []float64 {
	if g.Compressed {
		return make([]float64, g.NumActive())
	}

	return make([]float64, g.Nx*g.Ny*g.Nz)
}

// FieldSize returns the length of pressure arrays for this grid.
func (g *IBMGrid) FieldSize() int {
	if g.Compressed {
		return g.NumActive()
	}

	return g.Nx * g.Ny * g.Nz
}

// nodeIndex returns the flat index for grid coordinates (ix,iy,iz).
func (g *IBMGrid) nodeIndex(ix, iy, iz int) int {
	return ix*g.Ny*g.Nz + iy*g.Nz + iz
}

// nodePos returns the world position of grid node (ix,iy,iz).
//
// The offsets are rounded explicitly before being added to the origin.  Go is
// free to contract `origin + i*h` into a single FMA, which arm64 does and amd64
// does not, and an unrounded intermediate here shifts every node position by a
// few ulps — enough to flip the classification of a wall that sits on a node
// plane.  See offsetFromOrigin.
func (g *IBMGrid) nodePos(ix, iy, iz int) geometry.Vec3 {
	return geometry.Vec3{
		X: offsetFromOrigin(g.Origin.X, ix, g.H),
		Y: offsetFromOrigin(g.Origin.Y, iy, g.H),
		Z: offsetFromOrigin(g.Origin.Z, iz, g.H),
	}
}

// offsetFromOrigin returns base + i*h with the product rounded to float64
// before the addition, so the result cannot depend on FMA contraction.
// The Go spec guarantees that an explicit floating-point conversion rounds to
// the target precision, which is what prevents the fusion.
func offsetFromOrigin(base float64, i int, h float64) float64 {
	return base + float64(float64(i)*h)
}

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

	// Rounded explicitly, for the reason given on offsetFromOrigin: contracting
	// `center - half*h` into an FMA moves the origin by ~18 ulps on arm64
	// relative to amd64, which is enough to decide a node-plane tie either way.
	origin := geometry.Vec3{
		X: offsetFromOrigin(center.X, -halfNx, h),
		Y: offsetFromOrigin(center.Y, -halfNy, h),
		Z: offsetFromOrigin(center.Z, -halfNz, h),
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
							// Neighbour is exterior, so a wall lies between the
							// two nodes.  fracToWall never reports 0 here, which
							// is what keeps the sentinel unambiguous.
							frac, _ := fracToWall(room, p, dir, h)
							bi.Frac[a][d] = frac
						}
					}
				}

				g.Boundary[idx] = bi
			}
		}
	}

	// Build sparse iteration lists.
	g.buildActiveNodeLists()

	return g
}

// buildActiveNodeLists populates InteriorIdx and BoundaryIdx from the
// Class array for sparse iteration.
func (g *IBMGrid) buildActiveNodeLists() {
	nyNz := g.Ny * g.Nz

	g.InteriorIdx = g.InteriorIdx[:0]
	g.BoundaryIdx = g.BoundaryIdx[:0]

	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := ix*nyNz + iy*g.Nz + iz

				switch g.Class[idx] {
				case Interior:
					g.InteriorIdx = append(g.InteriorIdx, idx)
				case Boundary:
					g.BoundaryIdx = append(g.BoundaryIdx, boundaryNodeRef{
						Idx: idx, Ix: ix, Iy: iy, Iz: iz,
						Info: g.Boundary[idx],
					})
				}
			}
		}
	}
}

// nodePlaneTol is the relative window within which a wall is taken to sit
// exactly one cell away rather than a hair under or over it.
//
// Rooms are authored in round numbers, so a wall landing exactly on a node
// plane is the common case, not a corner case: every dimension that is an exact
// multiple of h produces one.  Whether the divided-out fraction then evaluates
// to 0.9999999999999964 or 1.0000000000000142 is decided by the last few ulps
// of the origin, which differ between amd64 and arm64.  Snapping the whole
// window to exactly 1 takes that decision away from the rounding mode.
//
// The tolerance is far above the ulp noise it absorbs (~1e-14 relative) and far
// below any sub-cell fraction a real geometry produces, so it cannot swallow a
// genuine cut cell.
const nodePlaneTol = 1e-9

// minWallFrac floors the sub-cell fraction for a wall that all but touches a
// node.  Frac feeds CFLLimit as dt ∝ sqrt(Frac), so an unbounded fraction drives
// the timestep to zero; and Frac must stay strictly positive to remain
// distinguishable from the "no wall here" sentinel.  A wall this close to a node
// is a degenerate cut cell either way, so pinning it is the usual remedy.
const minWallFrac = 1e-6

// fracToWall finds the fractional distance from point p along direction dir to
// the nearest wall of the convex room, normalised by h.
//
// It is called only for directions whose neighbour node is exterior, which for
// a convex room means a wall does lie between p and that neighbour.  The result
// is therefore always in (0, 1]: a fraction above 1 can only come from rounding
// and is clamped, never turned into the "no wall here" sentinel.  Returning 0
// here used to make the stencil read the exterior neighbour's pressure, pinned
// at zero, which imposes a pressure-release wall no matter what WallBC says.
//
// ok is false only for degenerate input — no wall plane faces dir at all — in
// which case the caller falls back to a full cell, i.e. a rigid wall on the
// node plane, rather than to a soft one.
func fracToWall(room *ConvexRoom, p, dir geometry.Vec3, h float64) (frac float64, ok bool) {
	bestT := math.Inf(1)

	for _, w := range room.Walls {
		// dir is an axis unit vector, so this product is exact.
		denom := w.Normal.Dot(dir)
		if denom >= 0 {
			// Ray moves away from or parallel to this wall's interior side.
			continue
		}

		// t = (Distance - Normal·p) / (Normal·dir) = -sideOf(p) / denom.
		// Expressed through sideOf so that the distance to a wall and the
		// inside/outside test that produced this boundary node cannot disagree.
		t := -sideOf(w, p) / denom
		if t > 0 && t < bestT {
			bestT = t
		}
	}

	if math.IsInf(bestT, 1) {
		return 1, false
	}

	frac = bestT / h

	switch {
	case frac > 1-nodePlaneTol:
		// Wall on (or within rounding distance of) the next node plane.
		return 1, true
	case frac < minWallFrac:
		return minWallFrac, true
	default:
		return frac, true
	}
}
