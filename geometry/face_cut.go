package geometry

import (
	"fmt"
	"math"
	"sort"
)

// CutRectangularHoles triangulates face minus holes, all expressed in the
// two-dimensional basis of frame, and lifts the result back into world space.
// The emitted winding matches frame.Normal.
//
// Holes flush with one, two, three, or all four face edges are supported, which
// matters because the canonical case is a door standing on the floor: its lower
// edge is colinear with the wall's lower edge. A hole covering the whole face
// yields no triangles at all, correctly deleting the wall.
//
// The method is a guillotine grid rather than a ring around each hole. Every
// hole edge contributes a cut line, so each resulting cell is wholly inside one
// hole or wholly outside all of them and no cell needs clipping. Cells are
// deliberately not merged back into maximal rectangles: merging introduces
// T-junctions and breaks watertightness, while the unmerged grid shares
// complete edges by construction. A wall with one door yields at most eight
// cells, so the extra triangles do not matter.
func CutRectangularHoles(frame PlaneFrame, face Rect2, holes []Rect2, eps float64) ([]Triangle, error) {
	if !face.Valid(eps) {
		return nil, fmt.Errorf("face rectangle is degenerate: %+v", face)
	}

	err := validateHoles(face, holes, eps)
	if err != nil {
		return nil, err
	}

	uCuts := cutLines(face.UMin, face.UMax, holes, func(h Rect2) (float64, float64) { return h.UMin, h.UMax }, eps)
	vCuts := cutLines(face.VMin, face.VMax, holes, func(h Rect2) (float64, float64) { return h.VMin, h.VMax }, eps)

	triangles := make([]Triangle, 0, 2*(len(uCuts)-1)*(len(vCuts)-1))

	for uIndex := 0; uIndex+1 < len(uCuts); uIndex++ {
		for vIndex := 0; vIndex+1 < len(vCuts); vIndex++ {
			cell := Rect2{UMin: uCuts[uIndex], VMin: vCuts[vIndex], UMax: uCuts[uIndex+1], VMax: vCuts[vIndex+1]}
			if !cell.Valid(eps) || cellIsInsideAnyHole(cell, holes, eps) {
				continue
			}

			triangles = append(triangles, cellTriangles(frame, cell)...)
		}
	}

	return triangles, nil
}

func validateHoles(face Rect2, holes []Rect2, eps float64) error {
	for index, hole := range holes {
		if !hole.Valid(eps) {
			return fmt.Errorf("hole %d is degenerate: %+v", index, hole)
		}

		if !face.Contains(hole, eps) {
			return fmt.Errorf("hole %d is not contained in the face", index)
		}

		for otherIndex := index + 1; otherIndex < len(holes); otherIndex++ {
			if hole.Overlaps(holes[otherIndex], eps) {
				return fmt.Errorf("holes %d and %d overlap", index, otherIndex)
			}
		}
	}

	return nil
}

// cutLines collects the sorted, deduplicated grid lines along one axis: the two
// face bounds plus both bounds of every hole, clamped to the face.
func cutLines(faceMin, faceMax float64, holes []Rect2, bounds func(Rect2) (float64, float64), eps float64) []float64 {
	values := make([]float64, 0, 2+2*len(holes))
	values = append(values, faceMin, faceMax)

	for _, hole := range holes {
		low, high := bounds(hole)
		values = append(values, min(max(low, faceMin), faceMax), min(max(high, faceMin), faceMax))
	}

	sort.Float64s(values)

	unique := values[:1]

	for _, value := range values[1:] {
		if value-unique[len(unique)-1] > eps {
			unique = append(unique, value)
		}
	}

	return unique
}

// cellIsInsideAnyHole tests the cell center, which is sufficient because every
// hole edge is a grid line, so no cell straddles a hole boundary.
func cellIsInsideAnyHole(cell Rect2, holes []Rect2, eps float64) bool {
	center := Vec2{U: (cell.UMin + cell.UMax) * 0.5, V: (cell.VMin + cell.VMax) * 0.5}
	for _, hole := range holes {
		if hole.ContainsPoint(center, -eps) {
			return true
		}
	}

	return false
}

// cellTriangles emits the two triangles of a grid cell wound so that their
// normal matches frame.Normal.
func cellTriangles(frame PlaneFrame, cell Rect2) []Triangle {
	a := frame.To3D(Vec2{U: cell.UMin, V: cell.VMin})
	b := frame.To3D(Vec2{U: cell.UMax, V: cell.VMin})
	c := frame.To3D(Vec2{U: cell.UMax, V: cell.VMax})
	d := frame.To3D(Vec2{U: cell.UMin, V: cell.VMax})

	// (U, V, Normal) is right-handed by construction, so U-then-V winding
	// already points along the normal.
	return []Triangle{
		{V0: a, V1: b, V2: c},
		{V0: a, V1: c, V2: d},
	}
}

// RectangleFromCoplanarPolygon projects a world-space planar polygon into frame
// and reports it as an axis-aligned rectangle. It fails when any vertex leaves
// the frame plane by more than eps or when the projection is not such a
// rectangle.
func RectangleFromCoplanarPolygon(frame PlaneFrame, polygon []Vec3, eps float64) (Rect2, bool) {
	projected := make([]Vec2, 0, len(polygon))

	for _, vertex := range polygon {
		if math.Abs(frame.Distance(vertex)) > eps {
			return Rect2{}, false
		}

		projected = append(projected, frame.To2D(vertex))
	}

	return Rect2FromPolygon(projected, eps)
}
