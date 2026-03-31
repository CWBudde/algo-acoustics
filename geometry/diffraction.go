package geometry

import (
	"math"
	"sort"
)

const (
	diffractionAngleEpsilon = 1e-9
	diffractionMergeEpsilon = 1e-8
	diffractionKeyScale     = 1e6
)

// DiffractionEdge describes one finite edge that can contribute first-order
// UTD diffraction.
type DiffractionEdge struct {
	Start, End  Vec3
	Direction   Vec3
	Length      float64
	WedgeIndex  float64
	FaceONormal Vec3
	FaceNNormal Vec3
	FaceOID     int
	FaceNID     int
	LocalBasis  [3]Vec3
}

type diffractionFaceEdge struct {
	FaceID   int
	Normal   Vec3
	Start    Vec3
	End      Vec3
	Opposite Vec3
}

type diffractionLineKey struct {
	PlaneA diffractionNormalKey
	PlaneB diffractionNormalKey
	Dir    diffractionVecKey
	Off    diffractionVecKey
}

type diffractionNormalKey struct {
	X int64
	Y int64
	Z int64
}

type diffractionVecKey struct {
	X int64
	Y int64
	Z int64
}

type diffractionInterval struct {
	StartT float64
	EndT   float64
	Edge   DiffractionEdge
	Offset Vec3
	Dir    Vec3
}

// ExtractDiffractionEdges extracts convex diffracting edges from a triangle
// mesh and merges adjacent colinear segments that share the same two planes.
func ExtractDiffractionEdges(mesh *Mesh) []DiffractionEdge {
	if mesh == nil || len(mesh.Triangles) == 0 {
		return nil
	}

	adjacency := make(map[meshEdgeKey][]diffractionFaceEdge, len(mesh.Triangles)*3)
	vertexByKey := make(map[meshVertexKey]Vec3, len(mesh.Triangles)*3)

	for triIndex, tri := range mesh.Triangles {
		normal := tri.Normal()
		if normal.Norm() == 0 {
			continue
		}

		record := func(start, end, opposite Vec3) {
			key := newMeshEdgeKey(start, end)
			adjacency[key] = append(adjacency[key], diffractionFaceEdge{
				FaceID:   triIndex,
				Normal:   normal,
				Start:    start,
				End:      end,
				Opposite: opposite,
			})

			vertexByKey[newMeshVertexKey(start)] = start
			vertexByKey[newMeshVertexKey(end)] = end
		}

		record(tri.V0, tri.V1, tri.V2)
		record(tri.V1, tri.V2, tri.V0)
		record(tri.V2, tri.V0, tri.V1)
	}

	rawEdges := make([]DiffractionEdge, 0, len(adjacency))

	for edgeKey, faces := range adjacency {
		if len(faces) != 2 {
			continue
		}

		start, startOK := vertexByKey[edgeKey.A]

		end, endOK := vertexByKey[edgeKey.B]
		if !startOK || !endOK {
			continue
		}

		direction := end.Sub(start)

		length := direction.Norm()
		if length <= diffractionAngleEpsilon {
			continue
		}

		direction = direction.Scale(1 / length)
		faceO := faces[0]
		faceN := faces[1]

		n0 := faceO.Normal.Normalize()

		n1 := faceN.Normal.Normalize()
		if n0.Norm() == 0 || n1.Norm() == 0 {
			continue
		}

		dot := clampFloat64(n0.Dot(n1), -1, 1)

		theta := math.Acos(dot)
		if theta <= diffractionAngleEpsilon || math.Abs(math.Pi-theta) <= diffractionAngleEpsilon {
			continue
		}

		// Convex edges have the adjacent opposite vertices behind each face plane.
		d01 := n0.Dot(faceN.Opposite.Sub(faceO.Start))

		d10 := n1.Dot(faceO.Opposite.Sub(faceN.Start))
		if d01 >= -diffractionAngleEpsilon || d10 >= -diffractionAngleEpsilon {
			continue
		}

		exteriorAngle := math.Pi + theta

		basis2 := direction.Cross(n0).Normalize()
		if basis2.Norm() == 0 {
			continue
		}

		rawEdges = append(rawEdges, DiffractionEdge{
			Start:       start,
			End:         end,
			Direction:   direction,
			Length:      length,
			WedgeIndex:  exteriorAngle / math.Pi,
			FaceONormal: n0,
			FaceNNormal: n1,
			FaceOID:     faceO.FaceID,
			FaceNID:     faceN.FaceID,
			LocalBasis: [3]Vec3{
				direction,
				n0,
				basis2,
			},
		})
	}

	merged := mergeDiffractionEdges(rawEdges)
	sort.Slice(merged, func(i, j int) bool {
		return compareVec3Lex(merged[i].Start, merged[j].Start) < 0 ||
			(compareVec3Lex(merged[i].Start, merged[j].Start) == 0 && compareVec3Lex(merged[i].End, merged[j].End) < 0)
	})

	return merged
}

func mergeDiffractionEdges(edges []DiffractionEdge) []DiffractionEdge {
	if len(edges) <= 1 {
		return edges
	}

	groups := make(map[diffractionLineKey][]diffractionInterval, len(edges))

	for _, edge := range edges {
		dir := canonicalDirection(edge.Direction)
		offset := edge.Start.Sub(dir.Scale(edge.Start.Dot(dir)))
		key := diffractionLineKey{
			PlaneA: makeSortedNormalPair(edge.FaceONormal, edge.FaceNNormal)[0],
			PlaneB: makeSortedNormalPair(edge.FaceONormal, edge.FaceNNormal)[1],
			Dir:    vecKeyFrom(dir),
			Off:    vecKeyFrom(offset),
		}

		t0 := edge.Start.Dot(dir)

		t1 := edge.End.Dot(dir)
		if t1 < t0 {
			t0, t1 = t1, t0
		}

		groups[key] = append(groups[key], diffractionInterval{
			StartT: t0,
			EndT:   t1,
			Edge:   edge,
			Offset: offset,
			Dir:    dir,
		})
	}

	merged := make([]DiffractionEdge, 0, len(edges))

	for _, intervals := range groups {
		sort.Slice(intervals, func(i, j int) bool {
			if intervals[i].StartT == intervals[j].StartT {
				return intervals[i].EndT < intervals[j].EndT
			}

			return intervals[i].StartT < intervals[j].StartT
		})

		current := intervals[0]
		currentStart := current.StartT
		currentEnd := current.EndT

		flush := func() {
			start := current.Dir.Scale(currentStart).Add(current.Offset)
			end := current.Dir.Scale(currentEnd).Add(current.Offset)
			edge := current.Edge
			edge.Start = start
			edge.End = end
			edge.Direction = end.Sub(start).Normalize()
			edge.Length = start.Distance(end)
			edge.LocalBasis = [3]Vec3{
				edge.Direction,
				edge.FaceONormal,
				edge.Direction.Cross(edge.FaceONormal).Normalize(),
			}
			merged = append(merged, edge)
		}

		for _, next := range intervals[1:] {
			if next.StartT <= currentEnd+diffractionMergeEpsilon {
				if next.EndT > currentEnd {
					currentEnd = next.EndT
				}

				continue
			}

			flush()

			current = next
			currentStart = next.StartT
			currentEnd = next.EndT
		}

		flush()
	}

	return merged
}

func makeSortedNormalPair(a, b Vec3) [2]diffractionNormalKey {
	keyA := normalKeyFrom(a)

	keyB := normalKeyFrom(b)
	if compareDiffractionNormalKey(keyA, keyB) <= 0 {
		return [2]diffractionNormalKey{keyA, keyB}
	}

	return [2]diffractionNormalKey{keyB, keyA}
}

func normalKeyFrom(v Vec3) diffractionNormalKey {
	v = v.Normalize()
	if v.X < 0 || (v.X == 0 && v.Y < 0) || (v.X == 0 && v.Y == 0 && v.Z < 0) {
		v = v.Neg()
	}

	return diffractionNormalKey{
		X: int64(math.Round(v.X * diffractionKeyScale)),
		Y: int64(math.Round(v.Y * diffractionKeyScale)),
		Z: int64(math.Round(v.Z * diffractionKeyScale)),
	}
}

func vecKeyFrom(v Vec3) diffractionVecKey {
	return diffractionVecKey{
		X: int64(math.Round(v.X * diffractionKeyScale)),
		Y: int64(math.Round(v.Y * diffractionKeyScale)),
		Z: int64(math.Round(v.Z * diffractionKeyScale)),
	}
}

func canonicalDirection(v Vec3) Vec3 {
	v = v.Normalize()
	if v.X < 0 || (v.X == 0 && v.Y < 0) || (v.X == 0 && v.Y == 0 && v.Z < 0) {
		return v.Neg()
	}

	return v
}

func compareDiffractionNormalKey(a, b diffractionNormalKey) int {
	switch {
	case a.X < b.X:
		return -1
	case a.X > b.X:
		return 1
	case a.Y < b.Y:
		return -1
	case a.Y > b.Y:
		return 1
	case a.Z < b.Z:
		return -1
	case a.Z > b.Z:
		return 1
	default:
		return 0
	}
}

func compareVec3Lex(a, b Vec3) int {
	switch {
	case a.X < b.X:
		return -1
	case a.X > b.X:
		return 1
	case a.Y < b.Y:
		return -1
	case a.Y > b.Y:
		return 1
	case a.Z < b.Z:
		return -1
	case a.Z > b.Z:
		return 1
	default:
		return 0
	}
}

func clampFloat64(v, minValue, maxValue float64) float64 {
	if v < minValue {
		return minValue
	}

	if v > maxValue {
		return maxValue
	}

	return v
}
