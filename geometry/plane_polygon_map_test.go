package geometry

import "testing"

// meshHouse builds an open "hut": four vertical walls (two triangles each) and
// a hip roof of four triangles meeting at an apex. That is 12 triangles lying
// on 8 distinct planes (4 wall planes + 4 roof planes).
func meshHouse() *Mesh {
	b0 := Vec3{X: 0, Y: 0, Z: 0}
	b1 := Vec3{X: 4, Y: 0, Z: 0}
	b2 := Vec3{X: 4, Y: 4, Z: 0}
	b3 := Vec3{X: 0, Y: 4, Z: 0}
	t0 := Vec3{X: 0, Y: 0, Z: 2}
	t1 := Vec3{X: 4, Y: 0, Z: 2}
	t2 := Vec3{X: 4, Y: 4, Z: 2}
	t3 := Vec3{X: 0, Y: 4, Z: 2}
	apex := Vec3{X: 2, Y: 2, Z: 3}

	quad := func(a, b, c, d Vec3) []Triangle {
		return []Triangle{{V0: a, V1: b, V2: c}, {V0: a, V1: c, V2: d}}
	}

	triangles := make([]Triangle, 0, 12)
	triangles = append(triangles, quad(b0, b1, t1, t0)...) // 0,1: y = 0 wall
	triangles = append(triangles, quad(b1, b2, t2, t1)...) // 2,3: x = 4 wall
	triangles = append(triangles, quad(b2, b3, t3, t2)...) // 4,5: y = 4 wall
	triangles = append(triangles, quad(b3, b0, t0, t3)...) // 6,7: x = 0 wall
	triangles = append(
		triangles,
		Triangle{V0: t0, V1: t1, V2: apex}, // 8
		Triangle{V0: t1, V1: t2, V2: apex}, // 9
		Triangle{V0: t2, V1: t3, V2: apex}, // 10
		Triangle{V0: t3, V1: t0, V2: apex}, // 11
	)

	return &Mesh{Triangles: triangles}
}

func TestBuildPlanePolygonMapTwelveTrianglesEightPlanes(t *testing.T) {
	t.Parallel()

	mesh := meshHouse()

	if len(mesh.Triangles) != 12 {
		t.Fatalf("fixture has %d triangles, want 12", len(mesh.Triangles))
	}

	ppm := BuildPlanePolygonMap(mesh)

	if ppm.PlaneCount() != 8 {
		t.Fatalf("PlaneCount() = %d, want 8", ppm.PlaneCount())
	}

	// Polygons must partition all triangle indices exactly once.
	seen := make(map[int]int, len(mesh.Triangles))

	for planeIndex := range ppm.PlaneCount() {
		for _, triIndex := range ppm.TrianglesOn(planeIndex) {
			seen[triIndex]++

			if got := ppm.PlaneOf(triIndex); got != planeIndex {
				t.Fatalf("PlaneOf(%d) = %d, want %d", triIndex, got, planeIndex)
			}
		}
	}

	if len(seen) != len(mesh.Triangles) {
		t.Fatalf("partition covers %d triangles, want %d", len(seen), len(mesh.Triangles))
	}

	for triIndex, count := range seen {
		if count != 1 {
			t.Fatalf("triangle %d appears %d times in the partition, want 1", triIndex, count)
		}
	}
}

func TestPlanePolygonMapSamePlane(t *testing.T) {
	t.Parallel()

	ppm := BuildPlanePolygonMap(meshHouse())

	tests := []struct {
		name string
		a, b int
		want bool
	}{
		{name: "same wall triangles", a: 0, b: 1, want: true},
		{name: "identity", a: 5, b: 5, want: true},
		{name: "across walls", a: 0, b: 2, want: false},
		{name: "wall vs roof", a: 0, b: 8, want: false},
		{name: "across roof slopes", a: 8, b: 9, want: false},
		{name: "out of range", a: 0, b: 99, want: false},
		{name: "both out of range", a: -1, b: -1, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ppm.SamePlane(tc.a, tc.b); got != tc.want {
				t.Fatalf("SamePlane(%d, %d) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestPlanePolygonMapContainsPoint(t *testing.T) {
	t.Parallel()

	mesh := meshHouse()
	ppm := BuildPlanePolygonMap(mesh)

	wallPlane := ppm.PlaneOf(0) // y = 0 wall
	roofPlane := ppm.PlaneOf(8)

	tests := []struct {
		name  string
		plane int
		point Vec3
		want  bool
	}{
		{name: "wall centre", plane: wallPlane, point: Vec3{X: 2, Y: 0, Z: 1}, want: true},
		{name: "inside first triangle", plane: wallPlane, point: Vec3{X: 3, Y: 0, Z: 0.5}, want: true},
		{name: "inside second triangle", plane: wallPlane, point: Vec3{X: 1, Y: 0, Z: 1.5}, want: true},
		{name: "coplanar but outside polygons", plane: wallPlane, point: Vec3{X: 6, Y: 0, Z: 1}, want: false},
		{name: "coplanar below polygons", plane: wallPlane, point: Vec3{X: 2, Y: 0, Z: -1}, want: false},
		{name: "off the plane", plane: wallPlane, point: Vec3{X: 2, Y: 1, Z: 1}, want: false},
		{name: "roof centroid", plane: roofPlane, point: mesh.Triangles[8].Centroid(), want: true},
		{name: "plane index out of range", plane: 99, point: Vec3{X: 2, Y: 0, Z: 1}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ppm.ContainsPoint(mesh, tc.plane, tc.point); got != tc.want {
				t.Fatalf("ContainsPoint(%d, %v) = %v, want %v", tc.plane, tc.point, got, tc.want)
			}
		})
	}
}

func TestBuildPlanePolygonMapBoxHasSixPlanes(t *testing.T) {
	t.Parallel()

	mesh := MeshFromBox(Vec3{X: 0, Y: 0, Z: 0}, Vec3{X: 2, Y: 3, Z: 4})

	ppm := BuildPlanePolygonMap(mesh)

	if ppm.PlaneCount() != 6 {
		t.Fatalf("PlaneCount() = %d, want 6", ppm.PlaneCount())
	}

	if ppm.TriangleCount() != 12 {
		t.Fatalf("TriangleCount() = %d, want 12", ppm.TriangleCount())
	}

	for planeIndex := range ppm.PlaneCount() {
		if got := len(ppm.TrianglesOn(planeIndex)); got != 2 {
			t.Fatalf("plane %d has %d triangles, want 2", planeIndex, got)
		}
	}
}

// Opposite-facing coplanar triangles must stay distinct planes, because the
// image-source mirroring is only valid for the side the normal points toward.
func TestBuildPlanePolygonMapOppositeNormalsAreDistinct(t *testing.T) {
	t.Parallel()

	front := Triangle{
		V0: Vec3{X: 0, Y: 0, Z: 0},
		V1: Vec3{X: 1, Y: 0, Z: 0},
		V2: Vec3{X: 0, Y: 1, Z: 0},
	}
	back := Triangle{V0: front.V0, V1: front.V2, V2: front.V1}

	ppm := BuildPlanePolygonMap(&Mesh{Triangles: []Triangle{front, back}})

	if ppm.PlaneCount() != 2 {
		t.Fatalf("PlaneCount() = %d, want 2 (opposite normals are distinct planes)", ppm.PlaneCount())
	}

	if ppm.SamePlane(0, 1) {
		t.Fatal("SamePlane(0, 1) = true, want false for opposite-facing coplanar triangles")
	}
}

func TestBuildPlanePolygonMapNearlyCoplanarWithinTolerance(t *testing.T) {
	t.Parallel()

	// Two triangles on z = 0, the second offset by far less than the 1e-6
	// distance tolerance, must be grouped together even though the exact
	// quantised keys may differ.
	a := Triangle{
		V0: Vec3{X: 0, Y: 0, Z: 0},
		V1: Vec3{X: 1, Y: 0, Z: 0},
		V2: Vec3{X: 0, Y: 1, Z: 0},
	}
	offset := 1e-12
	b := Triangle{
		V0: Vec3{X: 1, Y: 1, Z: offset},
		V1: Vec3{X: 0, Y: 1, Z: offset},
		V2: Vec3{X: 1, Y: 0, Z: offset},
	}

	ppm := BuildPlanePolygonMap(&Mesh{Triangles: []Triangle{a, b}})

	if ppm.PlaneCount() != 1 {
		t.Fatalf("PlaneCount() = %d, want 1", ppm.PlaneCount())
	}
}

func TestPlanePolygonMapNilSafety(t *testing.T) {
	t.Parallel()

	if got := BuildPlanePolygonMap(nil); got != nil {
		t.Fatalf("BuildPlanePolygonMap(nil) = %v, want nil", got)
	}

	var ppm *PlanePolygonMap

	if ppm.PlaneCount() != 0 {
		t.Fatal("nil PlaneCount() != 0")
	}

	if ppm.TriangleCount() != 0 {
		t.Fatal("nil TriangleCount() != 0")
	}

	if ppm.PlaneOf(0) != -1 {
		t.Fatal("nil PlaneOf(0) != -1")
	}

	if ppm.SamePlane(0, 0) {
		t.Fatal("nil SamePlane(0, 0) = true")
	}

	if ppm.TrianglesOn(0) != nil {
		t.Fatal("nil TrianglesOn(0) != nil")
	}

	if ppm.ContainsPoint(meshHouse(), 0, Vec3Zero) {
		t.Fatal("nil ContainsPoint(...) = true")
	}

	built := BuildPlanePolygonMap(meshHouse())
	if built.ContainsPoint(nil, 0, Vec3Zero) {
		t.Fatal("ContainsPoint(nil mesh) = true")
	}
}

func TestPointInTriangle(t *testing.T) {
	t.Parallel()

	tri := Triangle{
		V0: Vec3{X: 0, Y: 0, Z: 0},
		V1: Vec3{X: 2, Y: 0, Z: 0},
		V2: Vec3{X: 0, Y: 2, Z: 0},
	}
	degenerate := Triangle{
		V0: Vec3{X: 0, Y: 0, Z: 0},
		V1: Vec3{X: 1, Y: 0, Z: 0},
		V2: Vec3{X: 2, Y: 0, Z: 0},
	}

	tests := []struct {
		name  string
		tri   Triangle
		point Vec3
		want  bool
	}{
		{name: "interior", tri: tri, point: Vec3{X: 0.5, Y: 0.5, Z: 0}, want: true},
		{name: "vertex", tri: tri, point: tri.V1, want: true},
		{name: "edge midpoint", tri: tri, point: Vec3{X: 1, Y: 1, Z: 0}, want: true},
		{name: "outside in plane", tri: tri, point: Vec3{X: 2, Y: 2, Z: 0}, want: false},
		{name: "negative coordinate", tri: tri, point: Vec3{X: -0.5, Y: 0.5, Z: 0}, want: false},
		{name: "off plane", tri: tri, point: Vec3{X: 0.5, Y: 0.5, Z: 0.1}, want: false},
		{name: "degenerate triangle", tri: degenerate, point: Vec3{X: 1, Y: 0, Z: 0}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := pointInTriangle(tc.tri, tc.point, 1e-9); got != tc.want {
				t.Fatalf("pointInTriangle(%v) = %v, want %v", tc.point, got, tc.want)
			}
		})
	}
}

func BenchmarkBuildPlanePolygonMap(b *testing.B) {
	mesh := meshHouse()

	b.ResetTimer()

	for range b.N {
		_ = BuildPlanePolygonMap(mesh)
	}
}
