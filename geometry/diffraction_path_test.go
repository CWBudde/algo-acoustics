package geometry

import (
	"math"
	"testing"
)

func TestFindDiffractionPointOnEdge(t *testing.T) {
	edge := DiffractionEdge{
		Start:     Vec3{X: 0, Y: 0, Z: 0},
		End:       Vec3{X: 0, Y: 0, Z: 4},
		Direction: Vec3{Z: 1},
		Length:    4,
	}

	source := Vec3{X: 2, Y: 0, Z: 1}
	receiver := Vec3{X: -2, Y: 0, Z: 3}

	point, tParam, ok := FindDiffractionPoint(source, receiver, edge)
	if !ok {
		t.Fatal("FindDiffractionPoint() = false, want true")
	}

	want := Vec3{X: 0, Y: 0, Z: 2}
	if point.Distance(want) > 1e-12 {
		t.Fatalf("FindDiffractionPoint() point = %#v, want %#v", point, want)
	}

	if math.Abs(tParam-0.5) > 1e-12 {
		t.Fatalf("FindDiffractionPoint() t = %v, want 0.5", tParam)
	}
}

func TestFindDiffractionPointRejectsOutsideFiniteEdge(t *testing.T) {
	edge := DiffractionEdge{
		Start:     Vec3{X: 0, Y: 0, Z: 0},
		End:       Vec3{X: 0, Y: 0, Z: 1},
		Direction: Vec3{Z: 1},
		Length:    1,
	}

	source := Vec3{X: 2, Y: 0, Z: -1}
	receiver := Vec3{X: -2, Y: 0, Z: -1}

	if _, _, ok := FindDiffractionPoint(source, receiver, edge); ok {
		t.Fatal("FindDiffractionPoint() = true, want false for point outside edge segment")
	}
}

func TestDiffractionPathEnumerationFindsBarrierTopEdge(t *testing.T) {
	mesh := barrierMesh()
	edges := ExtractDiffractionEdges(mesh)

	source := Vec3{X: -6.5085842324344085, Y: 1.921311494612064, Z: 1.5096748432605978}
	receiver := Vec3{X: 4.20577077742135, Y: -1.6274612034653257, Z: 1.5365044111705466}

	paths := EnumerateDiffractionPaths(source, receiver, edges, mesh)
	if len(paths) == 0 {
		t.Fatal("EnumerateDiffractionPaths() returned no paths, want at least one")
	}

	wantPoint := Vec3{X: -0.1, Y: -1, Z: 1.526258714882967}
	found := false

	for _, path := range paths {
		if path.Point.Distance(wantPoint) <= 1e-9 {
			found = true

			if !PathVisible(mesh, source, path.Point, receiver) {
				t.Fatalf("EnumerateDiffractionPaths() returned an occluded path: %#v", path)
			}

			if math.Abs(path.TotalDistance-(source.Distance(path.Point)+receiver.Distance(path.Point))) > 1e-12 {
				t.Fatalf("TotalDistance = %v, want source+receiver path length", path.TotalDistance)
			}

			break
		}
	}

	if !found {
		t.Fatalf("EnumerateDiffractionPaths() did not find expected barrier-top edge point %#v", wantPoint)
	}
}

func TestEnumerateSecondOrderDiffractionPathsFindsInteriorFermatPoints(t *testing.T) {
	edges := []DiffractionEdge{
		{Start: Vec3{}, End: Vec3{Z: 4}, Direction: Vec3{Z: 1}, Length: 4},
		{Start: Vec3{X: 4}, End: Vec3{X: 4, Z: 4}, Direction: Vec3{Z: 1}, Length: 4},
	}
	source := Vec3{X: -2, Y: 1, Z: 1}
	receiver := Vec3{X: 6, Y: 1, Z: 3}

	paths := EnumerateSecondOrderDiffractionPaths(source, receiver, edges, nil)
	if len(paths) == 0 {
		t.Fatal("EnumerateSecondOrderDiffractionPaths() returned no paths")
	}

	wantFirstZ := 1 + 2*math.Sqrt(5)/(4+2*math.Sqrt(5))
	wantSecondZ := 1 + 2*(math.Sqrt(5)+4)/(4+2*math.Sqrt(5))
	var found bool

	for _, path := range paths {
		if path.Edge1.Start == edges[0].Start && path.Edge2.Start == edges[1].Start {
			found = true

			if math.Abs(path.Point1.Z-wantFirstZ) > 1e-7 || math.Abs(path.Point2.Z-wantSecondZ) > 1e-7 {
				t.Fatalf("Fermat points = (%v, %v), want z=(%v, %v)", path.Point1, path.Point2, wantFirstZ, wantSecondZ)
			}

			wantTypes := [...]DiffractionSubpathType{DiffractionSubpathS2D, DiffractionSubpathD2D, DiffractionSubpathD2R}
			wantStarts := [...]Vec3{source, path.Point1, path.Point2}
			wantEnds := [...]Vec3{path.Point1, path.Point2, receiver}

			for index, subpath := range path.Subpaths {
				if subpath.Type != wantTypes[index] || subpath.Start != wantStarts[index] || subpath.End != wantEnds[index] {
					t.Fatalf("Subpaths[%d] = %#v, want type=%v start=%v end=%v", index, subpath, wantTypes[index], wantStarts[index], wantEnds[index])
				}

				if math.Abs(subpath.Distance-subpath.Start.Distance(subpath.End)) > 1e-12 {
					t.Fatalf("Subpaths[%d].Distance = %v, want endpoint distance", index, subpath.Distance)
				}
			}
		}
	}

	if !found {
		t.Fatal("ordered edge-1 to edge-2 path not found")
	}

	if got := EnumerateSecondOrderDiffractionPaths(source, receiver, edges[:1], nil); got != nil {
		t.Fatalf("one-edge enumeration = %#v, want nil", got)
	}
}

func barrierMesh() *Mesh {
	minCorner := Vec3{X: -0.1, Y: -1, Z: 0}
	maxCorner := Vec3{X: 0.1, Y: 1, Z: 2}
	v000 := Vec3{X: minCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v001 := Vec3{X: minCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v010 := Vec3{X: minCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v011 := Vec3{X: minCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}
	v100 := Vec3{X: maxCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v101 := Vec3{X: maxCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v110 := Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v111 := Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}

	return &Mesh{Triangles: []Triangle{
		{V0: v000, V1: v110, V2: v100},
		{V0: v000, V1: v010, V2: v110},
		{V0: v001, V1: v101, V2: v111},
		{V0: v001, V1: v111, V2: v011},
		{V0: v000, V1: v101, V2: v001},
		{V0: v000, V1: v100, V2: v101},
		{V0: v010, V1: v011, V2: v111},
		{V0: v010, V1: v111, V2: v110},
		{V0: v000, V1: v001, V2: v011},
		{V0: v000, V1: v011, V2: v010},
		{V0: v100, V1: v110, V2: v111},
		{V0: v100, V1: v111, V2: v101},
	}}
}
