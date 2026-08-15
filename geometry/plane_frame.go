package geometry

import "math"

// Vec2 is a point expressed in the two-dimensional basis of a PlaneFrame.
type Vec2 struct {
	U, V float64
}

// Rect2 is an axis-aligned rectangle in a PlaneFrame basis.
type Rect2 struct {
	UMin, VMin, UMax, VMax float64
}

// PlaneFrame is an orthonormal two-dimensional basis anchored on a plane. Every
// face-cutting operation is a two-dimensional problem, and carrying it out in
// three dimensions is where such algorithms go wrong, so callers project into a
// frame first and lift the result back afterwards.
//
// The basis is chosen deterministically from the normal alone. Emitted vertices
// feed the scene geometry hashes, so a basis that flipped between two runs on
// the same input would spuriously invalidate downstream caches.
type PlaneFrame struct {
	Origin, U, V, Normal Vec3
}

// NewPlaneFrame builds a frame anchored at origin whose third axis is normal.
// The in-plane axes are derived by crossing the normal with the coordinate axis
// it is least aligned with, breaking ties toward the lowest axis index so the
// result never depends on floating-point noise. A zero-length normal yields the
// zero frame.
func NewPlaneFrame(origin, normal Vec3) PlaneFrame {
	unitNormal := normal.Normalize()
	if unitNormal == Vec3Zero {
		return PlaneFrame{Origin: origin}
	}

	axis := leastAlignedAxis(unitNormal)

	u := axis.Cross(unitNormal).Normalize()
	if u == Vec3Zero {
		return PlaneFrame{Origin: origin}
	}

	return PlaneFrame{
		Origin: origin,
		U:      u,
		V:      unitNormal.Cross(u).Normalize(),
		Normal: unitNormal,
	}
}

// leastAlignedAxis returns the coordinate axis least parallel to direction.
// Ties resolve toward the lowest axis index, which keeps the choice stable for
// axis-aligned normals such as shoebox walls.
func leastAlignedAxis(direction Vec3) Vec3 {
	absX := math.Abs(direction.X)
	absY := math.Abs(direction.Y)
	absZ := math.Abs(direction.Z)

	if absX <= absY && absX <= absZ {
		return Vec3{X: 1}
	}

	if absY <= absZ {
		return Vec3{Y: 1}
	}

	return Vec3{Z: 1}
}

// To2D projects a world-space point into the frame basis. The component along
// the normal is discarded, so callers that care about planarity must test it
// separately with Distance.
func (f PlaneFrame) To2D(p Vec3) Vec2 {
	offset := p.Sub(f.Origin)

	return Vec2{U: offset.Dot(f.U), V: offset.Dot(f.V)}
}

// To3D lifts a frame-basis point back into world space.
func (f PlaneFrame) To3D(p Vec2) Vec3 {
	return f.Origin.Add(f.U.Scale(p.U)).Add(f.V.Scale(p.V))
}

// Distance returns the signed distance of a world-space point from the frame
// plane, positive on the normal side.
func (f PlaneFrame) Distance(p Vec3) float64 {
	return p.Sub(f.Origin).Dot(f.Normal)
}

// BoundingRect2 returns the smallest axis-aligned rectangle containing points.
// An empty slice yields the zero rectangle.
func BoundingRect2(points []Vec2) Rect2 {
	if len(points) == 0 {
		return Rect2{}
	}

	rect := Rect2{UMin: points[0].U, VMin: points[0].V, UMax: points[0].U, VMax: points[0].V}
	for _, point := range points[1:] {
		rect.UMin = min(rect.UMin, point.U)
		rect.VMin = min(rect.VMin, point.V)
		rect.UMax = max(rect.UMax, point.U)
		rect.VMax = max(rect.VMax, point.V)
	}

	return rect
}

// Rect2FromPolygon reports the polygon as an axis-aligned rectangle in the
// frame basis. It succeeds only when the polygon really is such a rectangle:
// every vertex must sit on the bounding rectangle's outline, all four corners
// must be present, and the rectangle must have positive extent in both axes.
func Rect2FromPolygon(points []Vec2, eps float64) (Rect2, bool) {
	if len(points) < 4 {
		return Rect2{}, false
	}

	rect := BoundingRect2(points)
	if !rect.Valid(eps) {
		return Rect2{}, false
	}

	corners := [4]bool{}

	for _, point := range points {
		onU := math.Abs(point.U-rect.UMin) <= eps || math.Abs(point.U-rect.UMax) <= eps
		onV := math.Abs(point.V-rect.VMin) <= eps || math.Abs(point.V-rect.VMax) <= eps

		if !onU || !onV {
			return Rect2{}, false
		}

		index := 0
		if math.Abs(point.U-rect.UMax) <= eps {
			index |= 1
		}

		if math.Abs(point.V-rect.VMax) <= eps {
			index |= 2
		}

		corners[index] = true
	}

	for _, seen := range corners {
		if !seen {
			return Rect2{}, false
		}
	}

	return rect, true
}

// Valid reports whether the rectangle has positive extent in both axes.
func (r Rect2) Valid(eps float64) bool {
	return r.UMax-r.UMin > eps && r.VMax-r.VMin > eps
}

// Contains reports whether other lies inside r, allowing shared edges.
func (r Rect2) Contains(other Rect2, eps float64) bool {
	return other.UMin >= r.UMin-eps && other.UMax <= r.UMax+eps &&
		other.VMin >= r.VMin-eps && other.VMax <= r.VMax+eps
}

// Overlaps reports whether r and other share positive area. Rectangles that
// only touch along an edge or corner do not overlap.
func (r Rect2) Overlaps(other Rect2, eps float64) bool {
	return min(r.UMax, other.UMax)-max(r.UMin, other.UMin) > eps &&
		min(r.VMax, other.VMax)-max(r.VMin, other.VMin) > eps
}

// ContainsPoint reports whether the point lies inside the rectangle, allowing
// points on the outline.
func (r Rect2) ContainsPoint(p Vec2, eps float64) bool {
	return p.U >= r.UMin-eps && p.U <= r.UMax+eps && p.V >= r.VMin-eps && p.V <= r.VMax+eps
}
