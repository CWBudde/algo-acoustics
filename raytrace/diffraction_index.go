package raytrace

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const diffractionIndexMinCellSize = 0.25

type diffractionEdgeRef struct {
	index int
	edge  geometry.DiffractionEdge
}

type diffractionCellKey struct {
	X int
	Y int
	Z int
}

// DiffractionEdgeIndex is a simple uniform-grid spatial index for diffracting edges.
type DiffractionEdgeIndex struct {
	CellSize float64
	Bounds   geometry.Box
	cells    map[diffractionCellKey][]diffractionEdgeRef
	edges    []geometry.DiffractionEdge
}

// NewDiffractionEdgeIndex builds a spatial index from mesh-extracted edges.
func NewDiffractionEdgeIndex(mesh *geometry.Mesh) *DiffractionEdgeIndex {
	edges := geometry.ExtractDiffractionEdges(mesh)
	if len(edges) == 0 {
		return nil
	}

	bounds := mesh.BoundingBox()
	dimensions := bounds.Dimensions()
	cellSize := math.Max(diffractionIndexMinCellSize, math.Max(dimensions.X, math.Max(dimensions.Y, dimensions.Z))/8)

	index := &DiffractionEdgeIndex{
		CellSize: cellSize,
		Bounds:   bounds,
		cells:    make(map[diffractionCellKey][]diffractionEdgeRef),
		edges:    append([]geometry.DiffractionEdge(nil), edges...),
	}

	for i, edge := range index.edges {
		index.insert(i, edge)
	}

	return index
}

// Candidates returns the edges whose expanded bounding cells overlap the query segment.
func (i *DiffractionEdgeIndex) Candidates(start, end geometry.Vec3, padding float64) []geometry.DiffractionEdge {
	if i == nil || len(i.edges) == 0 {
		return nil
	}

	if padding < 0 {
		padding = 0
	}

	minCorner := geometry.Vec3{
		X: math.Min(start.X, end.X) - padding,
		Y: math.Min(start.Y, end.Y) - padding,
		Z: math.Min(start.Z, end.Z) - padding,
	}
	maxCorner := geometry.Vec3{
		X: math.Max(start.X, end.X) + padding,
		Y: math.Max(start.Y, end.Y) + padding,
		Z: math.Max(start.Z, end.Z) + padding,
	}

	minKey := i.keyForPoint(minCorner)
	maxKey := i.keyForPoint(maxCorner)

	seen := make(map[int]struct{})
	candidates := make([]geometry.DiffractionEdge, 0)

	for x := minKey.X; x <= maxKey.X; x++ {
		for y := minKey.Y; y <= maxKey.Y; y++ {
			for z := minKey.Z; z <= maxKey.Z; z++ {
				for _, ref := range i.cells[diffractionCellKey{X: x, Y: y, Z: z}] {
					if _, ok := seen[ref.index]; ok {
						continue
					}

					edgeMin, edgeMax := edgeBounds(ref.edge)
					if !boxesOverlap(minCorner, maxCorner, edgeMin, edgeMax) {
						continue
					}

					seen[ref.index] = struct{}{}
					candidates = append(candidates, ref.edge)
				}
			}
		}
	}

	return candidates
}

func (i *DiffractionEdgeIndex) insert(index int, edge geometry.DiffractionEdge) {
	minCorner, maxCorner := edgeBounds(edge)
	minKey := i.keyForPoint(minCorner)
	maxKey := i.keyForPoint(maxCorner)

	for x := minKey.X; x <= maxKey.X; x++ {
		for y := minKey.Y; y <= maxKey.Y; y++ {
			for z := minKey.Z; z <= maxKey.Z; z++ {
				key := diffractionCellKey{X: x, Y: y, Z: z}
				i.cells[key] = append(i.cells[key], diffractionEdgeRef{index: index, edge: edge})
			}
		}
	}
}

func (i *DiffractionEdgeIndex) keyForPoint(point geometry.Vec3) diffractionCellKey {
	if i == nil || i.CellSize <= 0 {
		return diffractionCellKey{}
	}

	return diffractionCellKey{
		X: int(math.Floor((point.X - i.Bounds.Min.X) / i.CellSize)),
		Y: int(math.Floor((point.Y - i.Bounds.Min.Y) / i.CellSize)),
		Z: int(math.Floor((point.Z - i.Bounds.Min.Z) / i.CellSize)),
	}
}

func edgeBounds(edge geometry.DiffractionEdge) (minCorner, maxCorner geometry.Vec3) {
	minCorner = geometry.Vec3{
		X: math.Min(edge.Start.X, edge.End.X),
		Y: math.Min(edge.Start.Y, edge.End.Y),
		Z: math.Min(edge.Start.Z, edge.End.Z),
	}
	maxCorner = geometry.Vec3{
		X: math.Max(edge.Start.X, edge.End.X),
		Y: math.Max(edge.Start.Y, edge.End.Y),
		Z: math.Max(edge.Start.Z, edge.End.Z),
	}

	return minCorner, maxCorner
}

func boxesOverlap(aMin, aMax, bMin, bMax geometry.Vec3) bool {
	return aMin.X <= bMax.X && aMax.X >= bMin.X &&
		aMin.Y <= bMax.Y && aMax.Y >= bMin.Y &&
		aMin.Z <= bMax.Z && aMax.Z >= bMin.Z
}
