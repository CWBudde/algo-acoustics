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

func barrierMesh() *Mesh {
	min := Vec3{X: -0.1, Y: -1, Z: 0}
	max := Vec3{X: 0.1, Y: 1, Z: 2}
	v000 := Vec3{X: min.X, Y: min.Y, Z: min.Z}
	v001 := Vec3{X: min.X, Y: min.Y, Z: max.Z}
	v010 := Vec3{X: min.X, Y: max.Y, Z: min.Z}
	v011 := Vec3{X: min.X, Y: max.Y, Z: max.Z}
	v100 := Vec3{X: max.X, Y: min.Y, Z: min.Z}
	v101 := Vec3{X: max.X, Y: min.Y, Z: max.Z}
	v110 := Vec3{X: max.X, Y: max.Y, Z: min.Z}
	v111 := Vec3{X: max.X, Y: max.Y, Z: max.Z}

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

func cubeMesh(minCorner, maxCorner Vec3) *Mesh {
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
