package geometry

import (
	"math"
	"sort"
)

const bvhLeafTriangleCount = 4

// BVHNode is a node in a bounding volume hierarchy over a mesh.
type BVHNode struct {
	AABB Box

	Left, Right *BVHNode
	Triangles   []int

	mesh *Mesh
}

// BuildBVH constructs a midpoint-split BVH for mesh.
func BuildBVH(mesh *Mesh) *BVHNode {
	if mesh == nil || len(mesh.Triangles) == 0 {
		return nil
	}

	indices := make([]int, len(mesh.Triangles))
	for index := range mesh.Triangles {
		indices[index] = index
	}

	return buildBVHNode(mesh, indices)
}

// Intersect returns the nearest ray-triangle hit in the BVH.
func (n *BVHNode) Intersect(r Ray) (t float64, triIdx int, hit bool) {
	if n == nil || n.mesh == nil {
		return 0, 0, false
	}

	return n.intersectNearest(r, math.Inf(1))
}

func buildBVHNode(mesh *Mesh, indices []int) *BVHNode {
	if len(indices) == 0 {
		return nil
	}

	node := &BVHNode{
		AABB: bvhBoundsForTriangles(mesh, indices),
		mesh: mesh,
	}

	if len(indices) <= bvhLeafTriangleCount {
		node.Triangles = append([]int(nil), indices...)
		return node
	}

	centroidBounds := bvhCentroidBounds(mesh, indices)
	axis := bvhLongestAxis(centroidBounds.Dimensions())
	pivot := bvhAxisValue(centroidBounds.Center(), axis)

	leftIndices := make([]int, 0, len(indices)/2)

	rightIndices := make([]int, 0, len(indices)/2)
	for _, triIdx := range indices {
		if bvhAxisValue(mesh.Triangles[triIdx].Centroid(), axis) < pivot {
			leftIndices = append(leftIndices, triIdx)
		} else {
			rightIndices = append(rightIndices, triIdx)
		}
	}

	if len(leftIndices) == 0 || len(rightIndices) == 0 {
		sorted := append([]int(nil), indices...)
		sort.Slice(sorted, func(i, j int) bool {
			left := bvhAxisValue(mesh.Triangles[sorted[i]].Centroid(), axis)

			right := bvhAxisValue(mesh.Triangles[sorted[j]].Centroid(), axis)
			if left == right {
				return sorted[i] < sorted[j]
			}

			return left < right
		})

		mid := len(sorted) / 2
		leftIndices = append(leftIndices[:0], sorted[:mid]...)
		rightIndices = append(rightIndices[:0], sorted[mid:]...)
	}

	node.Left = buildBVHNode(mesh, leftIndices)

	node.Right = buildBVHNode(mesh, rightIndices)
	if node.Left == nil || node.Right == nil {
		node.Left = nil
		node.Right = nil

		node.Triangles = append([]int(nil), indices...)
	}

	return node
}

func (n *BVHNode) intersectNearest(r Ray, maxT float64) (t float64, triIdx int, hit bool) {
	if n == nil || n.mesh == nil {
		return 0, 0, false
	}

	tMin, _, boxHit := RayBox(r, n.AABB)
	if !boxHit || tMin > maxT {
		return 0, 0, false
	}

	if len(n.Triangles) > 0 || (n.Left == nil && n.Right == nil) {
		return n.intersectLeaf(r, maxT)
	}

	children := [2]bvhTraversalCandidate{
		bvhTraversalForChild(n.Left, r),
		bvhTraversalForChild(n.Right, r),
	}
	if children[1].hit && (!children[0].hit || children[1].entry < children[0].entry) {
		children[0], children[1] = children[1], children[0]
	}

	return intersectBVHChildren(children, r, maxT)
}

func (n *BVHNode) intersectLeaf(r Ray, maxT float64) (float64, int, bool) {
	bestT := maxT
	bestIdx := -1

	for _, candidate := range n.Triangles {
		candidateT, candidateHit := RayTriangle(r, n.mesh.Triangles[candidate])
		if candidateHit && candidateT < bestT {
			bestT = candidateT
			bestIdx = candidate
		}
	}

	return bvhHitResult(bestT, bestIdx)
}

func intersectBVHChildren(children [2]bvhTraversalCandidate, r Ray, maxT float64) (float64, int, bool) {
	bestT := maxT
	bestIdx := -1

	for _, child := range children {
		if !child.hit || child.entry > bestT {
			continue
		}

		candidateT, candidateIdx, candidateHit := child.node.intersectNearest(r, bestT)
		if candidateHit && candidateT < bestT {
			bestT = candidateT
			bestIdx = candidateIdx
		}
	}

	return bvhHitResult(bestT, bestIdx)
}

func bvhHitResult(t float64, triangleIndex int) (float64, int, bool) {
	if triangleIndex < 0 {
		return 0, 0, false
	}

	return t, triangleIndex, true
}

type bvhTraversalCandidate struct {
	node  *BVHNode
	entry float64
	hit   bool
}

func bvhTraversalForChild(node *BVHNode, r Ray) bvhTraversalCandidate {
	if node == nil {
		return bvhTraversalCandidate{}
	}

	tMin, _, hit := RayBox(r, node.AABB)
	if !hit {
		return bvhTraversalCandidate{}
	}

	return bvhTraversalCandidate{node: node, entry: max64(0, tMin), hit: true}
}

func bvhBoundsForTriangles(mesh *Mesh, indices []int) Box {
	bounds := bvhTriangleBounds(mesh.Triangles[indices[0]])
	for _, triIdx := range indices[1:] {
		bounds = bvhUnionBox(bounds, bvhTriangleBounds(mesh.Triangles[triIdx]))
	}

	return bounds
}

func bvhCentroidBounds(mesh *Mesh, indices []int) Box {
	centroid := mesh.Triangles[indices[0]].Centroid()

	bounds := Box{Min: centroid, Max: centroid}
	for _, triIdx := range indices[1:] {
		centroid = mesh.Triangles[triIdx].Centroid()
		bounds.Min = Vec3{
			X: min64(bounds.Min.X, centroid.X),
			Y: min64(bounds.Min.Y, centroid.Y),
			Z: min64(bounds.Min.Z, centroid.Z),
		}
		bounds.Max = Vec3{
			X: max64(bounds.Max.X, centroid.X),
			Y: max64(bounds.Max.Y, centroid.Y),
			Z: max64(bounds.Max.Z, centroid.Z),
		}
	}

	return bounds
}

func bvhTriangleBounds(tri Triangle) Box {
	minCorner := tri.V0

	maxCorner := tri.V0
	for _, vertex := range []Vec3{tri.V1, tri.V2} {
		minCorner = Vec3{
			X: min64(minCorner.X, vertex.X),
			Y: min64(minCorner.Y, vertex.Y),
			Z: min64(minCorner.Z, vertex.Z),
		}
		maxCorner = Vec3{
			X: max64(maxCorner.X, vertex.X),
			Y: max64(maxCorner.Y, vertex.Y),
			Z: max64(maxCorner.Z, vertex.Z),
		}
	}

	return Box{Min: minCorner, Max: maxCorner}
}

func bvhUnionBox(a, b Box) Box {
	return Box{
		Min: Vec3{
			X: min64(a.Min.X, b.Min.X),
			Y: min64(a.Min.Y, b.Min.Y),
			Z: min64(a.Min.Z, b.Min.Z),
		},
		Max: Vec3{
			X: max64(a.Max.X, b.Max.X),
			Y: max64(a.Max.Y, b.Max.Y),
			Z: max64(a.Max.Z, b.Max.Z),
		},
	}
}

func bvhLongestAxis(v Vec3) int {
	switch {
	case v.Y > v.X && v.Y >= v.Z:
		return 1
	case v.Z > v.X && v.Z > v.Y:
		return 2
	default:
		return 0
	}
}

func bvhAxisValue(v Vec3, axis int) float64 {
	switch axis {
	case 1:
		return v.Y
	case 2:
		return v.Z
	default:
		return v.X
	}
}
