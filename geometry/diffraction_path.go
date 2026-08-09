package geometry

import (
	"sort"
)

const diffractionPathEpsilon = 1e-9

const (
	secondOrderIterations = 48
	goldenSectionRatio    = 0.6180339887498948482
)

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

// DiffractionSubpathType identifies one segment of a multi-edge diffraction
// path using the RAVEN S2D, D2D, and D2R decomposition.
type DiffractionSubpathType int

const (
	DiffractionSubpathS2D DiffractionSubpathType = iota
	DiffractionSubpathD2D
	DiffractionSubpathD2R
)

// DiffractionSubpath describes one physical segment of a multi-edge path.
type DiffractionSubpath struct {
	Type     DiffractionSubpathType
	Start    Vec3
	End      Vec3
	Distance float64
}

// SecondOrderDiffractionPath describes a path that bends successively around
// two distinct finite edges. Point1 and Point2 are the joint Fermat minimum of
// the three-segment path over the two edge intervals.
type SecondOrderDiffractionPath struct {
	Source           Vec3
	Receiver         Vec3
	Point1           Vec3
	Point2           Vec3
	Edge1            DiffractionEdge
	Edge2            DiffractionEdge
	SourceDistance   float64
	EdgeDistance     float64
	ReceiverDistance float64
	TotalDistance    float64
	Subpaths         [3]DiffractionSubpath
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

// SegmentVisible reports whether a finite line segment is unobstructed by the
// mesh. Intersections at the segment endpoint are permitted.
func SegmentVisible(mesh *Mesh, start, end Vec3) bool {
	return segmentVisible(BuildBVH(mesh), start, end)
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

// EnumerateSecondOrderDiffractionPaths returns visible ordered paths through
// pairs of distinct finite edges. The edge coordinates are found by bounded
// coordinate minimization of the convex broken-path length. Minima on edge
// endpoints are rejected because endpoints are not valid diffraction sources.
//
//nolint:cyclop // Ordered-pair validation is clearer as one deterministic enumeration loop.
func EnumerateSecondOrderDiffractionPaths(source, receiver Vec3, edges []DiffractionEdge, mesh *Mesh) []SecondOrderDiffractionPath {
	if len(edges) < 2 {
		return nil
	}

	bvh := BuildBVH(mesh)
	paths := make([]SecondOrderDiffractionPath, 0, len(edges)*(len(edges)-1))

	for firstIndex := range edges {
		for secondIndex := range edges {
			if firstIndex == secondIndex {
				continue
			}

			// #nosec G602 -- both indices are produced by range over edges.
			first := edges[firstIndex]
			second := edges[secondIndex]

			point1, point2, t1, t2, ok := findSecondOrderDiffractionPoints(source, receiver, first, second)
			if !ok || t1 <= diffractionPathEpsilon || t1 >= 1-diffractionPathEpsilon ||
				t2 <= diffractionPathEpsilon || t2 >= 1-diffractionPathEpsilon {
				continue
			}

			if !segmentVisible(bvh, source, point1) || !segmentVisible(bvh, point1, point2) ||
				!segmentVisible(bvh, point2, receiver) {
				continue
			}

			sourceDistance := source.Distance(point1)
			edgeDistance := point1.Distance(point2)
			receiverDistance := point2.Distance(receiver)
			paths = append(paths, SecondOrderDiffractionPath{
				Source:           source,
				Receiver:         receiver,
				Point1:           point1,
				Point2:           point2,
				Edge1:            first,
				Edge2:            second,
				SourceDistance:   sourceDistance,
				EdgeDistance:     edgeDistance,
				ReceiverDistance: receiverDistance,
				TotalDistance:    sourceDistance + edgeDistance + receiverDistance,
				Subpaths: [3]DiffractionSubpath{
					{Type: DiffractionSubpathS2D, Start: source, End: point1, Distance: sourceDistance},
					{Type: DiffractionSubpathD2D, Start: point1, End: point2, Distance: edgeDistance},
					{Type: DiffractionSubpathD2R, Start: point2, End: receiver, Distance: receiverDistance},
				},
			})
		}
	}

	sort.Slice(paths, func(i, j int) bool {
		if paths[i].TotalDistance != paths[j].TotalDistance {
			return paths[i].TotalDistance < paths[j].TotalDistance
		}

		if comparison := compareVec3Lex(paths[i].Edge1.Start, paths[j].Edge1.Start); comparison != 0 {
			return comparison < 0
		}

		if comparison := compareVec3Lex(paths[i].Edge2.Start, paths[j].Edge2.Start); comparison != 0 {
			return comparison < 0
		}

		if comparison := compareVec3Lex(paths[i].Point1, paths[j].Point1); comparison != 0 {
			return comparison < 0
		}

		return compareVec3Lex(paths[i].Point2, paths[j].Point2) < 0
	})

	return paths
}

func findSecondOrderDiffractionPoints(source, receiver Vec3, first, second DiffractionEdge) (point1, point2 Vec3, t1, t2 float64, ok bool) {
	if first.Length <= diffractionPathEpsilon || second.Length <= diffractionPathEpsilon {
		return Vec3{}, Vec3{}, 0, 0, false
	}

	direction1 := first.Direction.Normalize()
	direction2 := second.Direction.Normalize()

	if direction1 == Vec3Zero || direction2 == Vec3Zero {
		return Vec3{}, Vec3{}, 0, 0, false
	}

	pointAt1 := func(t float64) Vec3 { return first.Start.Add(direction1.Scale(t * first.Length)) }
	pointAt2 := func(t float64) Vec3 { return second.Start.Add(direction2.Scale(t * second.Length)) }

	// The objective is convex on the unit square. Alternating exact 1-D
	// golden-section minimizations therefore converges deterministically.
	t1, t2 = 0.5, 0.5
	for range secondOrderIterations {
		fixed2 := pointAt2(t2)
		t1 = goldenSectionMinimum(func(candidate float64) float64 {
			point := pointAt1(candidate)

			return source.Distance(point) + point.Distance(fixed2)
		})

		fixed1 := pointAt1(t1)
		t2 = goldenSectionMinimum(func(candidate float64) float64 {
			point := pointAt2(candidate)

			return fixed1.Distance(point) + point.Distance(receiver)
		})
	}

	point1, point2 = pointAt1(t1), pointAt2(t2)
	if source.Distance(point1) <= diffractionPathEpsilon || point1.Distance(point2) <= diffractionPathEpsilon ||
		point2.Distance(receiver) <= diffractionPathEpsilon {
		return Vec3{}, Vec3{}, 0, 0, false
	}

	return point1, point2, t1, t2, true
}

func goldenSectionMinimum(objective func(float64) float64) float64 {
	left, right := 0.0, 1.0
	c := right - goldenSectionRatio*(right-left)
	d := left + goldenSectionRatio*(right-left)
	fc, fd := objective(c), objective(d)

	for range secondOrderIterations {
		if fc < fd {
			right, d, fd = d, c, fc
			c = right - goldenSectionRatio*(right-left)
			fc = objective(c)
		} else {
			left, c, fc = c, d, fd
			d = left + goldenSectionRatio*(right-left)
			fd = objective(d)
		}
	}

	return (left + right) / 2
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
