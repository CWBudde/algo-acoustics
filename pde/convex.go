package pde

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// ConvexRoom represents a convex polyhedral room defined by wall half-planes.
// Each wall plane has an inward-pointing normal so that interior points
// satisfy n·p − d > 0 for every wall.
type ConvexRoom struct {
	Walls    []geometry.Plane // inward-pointing unit normals
	Vertices []geometry.Vec3  // vertices of the polyhedron
}

// NewConvexRoom creates a ConvexRoom and validates that the geometry is convex:
// every vertex must lie on the non-negative side of every wall plane.
func NewConvexRoom(walls []geometry.Plane, vertices []geometry.Vec3) (*ConvexRoom, error) {
	if len(walls) < 4 {
		return nil, fmt.Errorf("convex room needs at least 4 walls, got %d", len(walls))
	}

	if len(vertices) < 4 {
		return nil, fmt.Errorf("convex room needs at least 4 vertices, got %d", len(vertices))
	}

	const tol = 1e-9

	for vi, v := range vertices {
		for wi, w := range walls {
			if s := w.SideOf(v); s < -tol {
				return nil, fmt.Errorf(
					"vertex %d (%.4g, %.4g, %.4g) is on wrong side of wall %d (side=%.6g)",
					vi, v.X, v.Y, v.Z, wi, s,
				)
			}
		}
	}

	r := &ConvexRoom{
		Walls:    make([]geometry.Plane, len(walls)),
		Vertices: make([]geometry.Vec3, len(vertices)),
	}
	copy(r.Walls, walls)
	copy(r.Vertices, vertices)

	return r, nil
}

// PointInside reports whether p is strictly inside the convex room.
// A point is inside when it is on the positive side of every wall plane.
func (r *ConvexRoom) PointInside(p geometry.Vec3) bool {
	for _, w := range r.Walls {
		if w.SideOf(p) <= 0 {
			return false
		}
	}

	return true
}

// WallDistance holds the result of a nearest-wall query.
type WallDistance struct {
	Dist    float64       // perpendicular distance to the wall
	Normal  geometry.Vec3 // inward-pointing wall normal
	WallIdx int           // index into ConvexRoom.Walls
}

// DistanceToNearestWall returns the perpendicular distance from p to the
// closest wall plane, together with that wall's inward normal and index.
// For interior points the distance is positive; for exterior points it is
// the signed distance to the nearest wall (negative means outside).
func (r *ConvexRoom) DistanceToNearestWall(p geometry.Vec3) WallDistance {
	best := WallDistance{Dist: math.Inf(1), WallIdx: -1}

	for i, w := range r.Walls {
		d := w.SideOf(p) // positive = inside this half-plane
		if d < best.Dist {
			best = WallDistance{Dist: d, Normal: w.Normal, WallIdx: i}
		}
	}

	return best
}

// BoundingBox returns the axis-aligned bounding box of the room vertices,
// expanded by padding on every side (e.g. for PML absorbing layers).
func (r *ConvexRoom) BoundingBox(padding float64) geometry.Box {
	if len(r.Vertices) == 0 {
		return geometry.Box{}
	}

	lo := r.Vertices[0]
	hi := r.Vertices[0]

	for _, v := range r.Vertices[1:] {
		lo.X = math.Min(lo.X, v.X)
		lo.Y = math.Min(lo.Y, v.Y)
		lo.Z = math.Min(lo.Z, v.Z)
		hi.X = math.Max(hi.X, v.X)
		hi.Y = math.Max(hi.Y, v.Y)
		hi.Z = math.Max(hi.Z, v.Z)
	}

	pad := geometry.Vec3{X: padding, Y: padding, Z: padding}

	return geometry.NewBox(lo.Sub(pad), hi.Add(pad))
}
