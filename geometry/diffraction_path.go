package geometry

import (
	"sort"
)

const diffractionPathEpsilon = 1e-9

// DiffractionPath describes a first-order path that bends around one edge.
type DiffractionPath struct {
	Source           Vec3
	Receiver         Vec3
	Point            Vec3
	Edge             DiffractionEdge
	SourceDistance   float64
	ReceiverDistance float64
	TotalDistance    float64
}

// FindDiffractionPoint returns the point on a finite edge that minimizes the
// broken-path length |S - P| + |P - R|. The returned t parameter is normalized
// to [0, 1] over the finite edge segment.
func FindDiffractionPoint(source, receiver Vec3, edge DiffractionEdge) (point Vec3, t float64, ok bool) {
	if edge.Length <= diffractionPathEpsilon {
		return Vec3{}, 0, false
	}

	direction := edge.Direction.Normalize()
	if direction == Vec3Zero {
		return Vec3{}, 0, false
	}

	sourceRel := source.Sub(edge.Start)
	receiverRel := receiver.Sub(edge.Start)

	sourceParallel := sourceRel.Dot(direction)
	receiverParallel := receiverRel.Dot(direction)

	sourcePerp := sourceRel.Sub(direction.Scale(sourceParallel))
	receiverPerp := receiverRel.Sub(direction.Scale(receiverParallel))

	sourcePerpDistance := sourcePerp.Norm()
	receiverPerpDistance := receiverPerp.Norm()
	if sourcePerpDistance <= diffractionPathEpsilon || receiverPerpDistance <= diffractionPathEpsilon {
		return Vec3{}, 0, false
	}

	edgeCoordinate := (sourceParallel*receiverPerpDistance + receiverParallel*sourcePerpDistance) / (sourcePerpDistance + receiverPerpDistance)
	if edgeCoordinate < -diffractionPathEpsilon || edgeCoordinate > edge.Length+diffractionPathEpsilon {
		return Vec3{}, 0, false
	}

	point = edge.Start.Add(direction.Scale(edgeCoordinate))
	t = edgeCoordinate / edge.Length
	return point, t, true
}

// PathVisible reports whether the two line segments source->point and
// point->receiver are unobstructed by the mesh.
func PathVisible(mesh *Mesh, source, point, receiver Vec3) bool {
	if mesh == nil || len(mesh.Triangles) == 0 {
		return true
	}

	bvh := BuildBVH(mesh)
	return segmentVisible(bvh, source, point) && segmentVisible(bvh, point, receiver)
}

// EnumerateDiffractionPaths returns the first-order diffraction paths that are
// valid for the given mesh and edge set.
func EnumerateDiffractionPaths(source, receiver Vec3, edges []DiffractionEdge, mesh *Mesh) []DiffractionPath {
	if len(edges) == 0 {
		return nil
	}

	bvh := BuildBVH(mesh)
	paths := make([]DiffractionPath, 0, len(edges))
	for _, edge := range edges {
		point, _, ok := FindDiffractionPoint(source, receiver, edge)
		if !ok {
			continue
		}

		if !segmentVisible(bvh, source, point) || !segmentVisible(bvh, point, receiver) {
			continue
		}

		sourceDistance := source.Distance(point)
		receiverDistance := receiver.Distance(point)
		paths = append(paths, DiffractionPath{
			Source:           source,
			Receiver:         receiver,
			Point:            point,
			Edge:             edge,
			SourceDistance:   sourceDistance,
			ReceiverDistance: receiverDistance,
			TotalDistance:    sourceDistance + receiverDistance,
		})
	}

	sort.Slice(paths, func(i, j int) bool {
		if paths[i].TotalDistance != paths[j].TotalDistance {
			return paths[i].TotalDistance < paths[j].TotalDistance
		}

		if paths[i].Edge.Length != paths[j].Edge.Length {
			return paths[i].Edge.Length < paths[j].Edge.Length
		}

		return compareVec3Lex(paths[i].Point, paths[j].Point) < 0
	})

	return paths
}

func segmentVisible(bvh *BVHNode, start, end Vec3) bool {
	if start.Distance(end) <= diffractionPathEpsilon {
		return false
	}

	if bvh == nil {
		return true
	}

	direction := end.Sub(start)
	distance := direction.Norm()
	if distance <= diffractionPathEpsilon {
		return false
	}

	ray := Ray{Origin: start, Direction: direction.Scale(1 / distance)}
	t, _, hit := bvh.Intersect(ray)
	if !hit {
		return true
	}

	return t >= distance-diffractionPathEpsilon
}
