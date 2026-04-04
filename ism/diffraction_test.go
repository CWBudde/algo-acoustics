package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestEnumerateReflectionDiffractionPathsDelegatesToGeometry(t *testing.T) {
	mesh := cubeMesh(geometry.Vec3Zero, geometry.Vec3{X: 1, Y: 1, Z: 1})
	edges := geometry.ExtractDiffractionEdges(mesh)
	imageSources := []geometry.Vec3{{X: -2, Y: -2, Z: 0.2}}
	receiver := geometry.Vec3{X: -2, Y: -2, Z: 1}

	got := EnumerateReflectionDiffractionPaths(imageSources, receiver, edges, mesh)
	want := geometry.EnumerateDiffractionPaths(imageSources[0], receiver, edges, mesh)

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}

	if len(got) == 0 {
		t.Fatal("EnumerateReflectionDiffractionPaths() returned no paths, want at least one")
	}

	if got[0].Point.Distance(want[0].Point) > 1e-12 {
		t.Fatalf("first path point = %#v, want %#v", got[0].Point, want[0].Point)
	}
}

func cubeMesh(minCorner, maxCorner geometry.Vec3) *geometry.Mesh {
	v000 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v001 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v010 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v011 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}
	v100 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v101 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v110 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v111 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}

	return &geometry.Mesh{Triangles: []geometry.Triangle{
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
