package geometry_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestMeshFromBox(t *testing.T) {
	t.Parallel()

	bmin := geometry.Vec3{X: -1, Y: -1, Z: -1}
	bmax := geometry.Vec3{X: 1, Y: 1, Z: 1}
	mesh := geometry.MeshFromBox(bmin, bmax)

	if len(mesh.Triangles) != 12 {
		t.Fatalf("MeshFromBox() produced %d triangles, want 12", len(mesh.Triangles))
	}

	center := geometry.Vec3{X: 0, Y: 0, Z: 0}

	for i, tri := range mesh.Triangles {
		normal := tri.Normal()
		centroid := tri.Centroid()

		// Normal should point toward the center from the face.
		toCenter := center.Sub(centroid)
		if normal.Dot(toCenter) <= 0 {
			t.Errorf("triangle[%d]: normal %v does not point inward (centroid=%v)", i, normal, centroid)
		}
	}

	err := mesh.Validate()
	if err != nil {
		t.Fatalf("MeshFromBox() mesh validation failed: %v", err)
	}
}

func TestMeshBoundingBox(t *testing.T) {
	mesh := geometry.Mesh{Triangles: []geometry.Triangle{{
		V0: geometry.Vec3{X: -1, Y: 2, Z: 0},
		V1: geometry.Vec3{X: 4, Y: -3, Z: 5},
		V2: geometry.Vec3{X: 2, Y: 1, Z: -2},
	}}}

	got := mesh.BoundingBox()
	want := geometry.NewBox(
		geometry.Vec3{X: -1, Y: -3, Z: -2},
		geometry.Vec3{X: 4, Y: 2, Z: 5},
	)

	if got != want {
		t.Fatalf("BoundingBox() = %#v, want %#v", got, want)
	}
}

func TestMeshValidateClosedMesh(t *testing.T) {
	mesh := geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: geometry.Vec3{0, 0, 0}, V1: geometry.Vec3{1, 0, 0}, V2: geometry.Vec3{0, 1, 0}},
		{V0: geometry.Vec3{0, 0, 0}, V1: geometry.Vec3{0, 0, 1}, V2: geometry.Vec3{1, 0, 0}},
		{V0: geometry.Vec3{0, 0, 0}, V1: geometry.Vec3{0, 1, 0}, V2: geometry.Vec3{0, 0, 1}},
		{V0: geometry.Vec3{1, 0, 0}, V1: geometry.Vec3{0, 0, 1}, V2: geometry.Vec3{0, 1, 0}},
	}}

	err := mesh.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestMeshValidateDegenerateTriangle(t *testing.T) {
	mesh := geometry.Mesh{Triangles: []geometry.Triangle{{
		V0: geometry.Vec3{0, 0, 0},
		V1: geometry.Vec3{1, 0, 0},
		V2: geometry.Vec3{2, 0, 0},
	}}}

	err := mesh.Validate()
	if err == nil {
		t.Fatal("Validate() returned nil for degenerate triangle")
	}

	var issues *geometry.MeshValidationIssues
	if !errors.As(err, &issues) {
		t.Fatalf("Validate() error type = %T, want *geometry.MeshValidationIssues", err)
	}

	if !issues.HasProblems() {
		t.Fatal("Validate() should report a hard problem for degenerate geometry")
	}

	if !strings.Contains(err.Error(), "triangle[0] is degenerate") {
		t.Fatalf("Validate() = %q, want degenerate triangle message", err)
	}
}

func TestMeshValidateRejectsNonFiniteVertices(t *testing.T) {
	tests := []struct {
		name  string
		value float64
	}{
		{name: "NaN", value: math.NaN()},
		{name: "positive infinity", value: math.Inf(1)},
		{name: "negative infinity", value: math.Inf(-1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mesh := geometry.Mesh{Triangles: []geometry.Triangle{{
				V0: geometry.Vec3{X: test.value},
				V1: geometry.Vec3{X: 1},
				V2: geometry.Vec3{Y: 1},
			}}}

			err := mesh.Validate()

			var issues *geometry.MeshValidationIssues
			if !errors.As(err, &issues) || !issues.HasProblems() {
				t.Fatalf("Validate() error = %v, want hard mesh validation issue", err)
			}

			if !strings.Contains(err.Error(), "non-finite vertex") {
				t.Fatalf("Validate() error = %v, want non-finite vertex message", err)
			}
		})
	}
}

func TestMeshValidateWarnsNonWatertight(t *testing.T) {
	mesh := geometry.Mesh{Triangles: []geometry.Triangle{{
		V0: geometry.Vec3{0, 0, 0},
		V1: geometry.Vec3{1, 0, 0},
		V2: geometry.Vec3{0, 1, 0},
	}}}

	err := mesh.Validate()
	if err == nil {
		t.Fatal("Validate() returned nil for open mesh")
	}

	var issues *geometry.MeshValidationIssues
	if !errors.As(err, &issues) {
		t.Fatalf("Validate() error type = %T, want *geometry.MeshValidationIssues", err)
	}

	if issues.HasProblems() {
		t.Fatalf("Validate() should only warn for non-watertight mesh, got %v", err)
	}

	if !strings.Contains(err.Error(), "not watertight") {
		t.Fatalf("Validate() = %q, want watertight warning", err)
	}
}

func TestLoadOBJTriangle(t *testing.T) {
	mesh := loadOBJFixture(t, "triangle.obj", strings.Join([]string{
		"v 0 0 0",
		"v 1 0 0",
		"v 0 1 0",
		"f 1 2 3",
	}, "\n"))

	if len(mesh.Triangles) != 1 {
		t.Fatalf("len(Triangles) = %d, want 1", len(mesh.Triangles))
	}

	tri := mesh.Triangles[0]
	if tri.V0 != (geometry.Vec3{0, 0, 0}) || tri.V1 != (geometry.Vec3{1, 0, 0}) || tri.V2 != (geometry.Vec3{0, 1, 0}) {
		t.Fatalf("parsed triangle = %#v, want original vertex order", tri)
	}
}

func TestLoadOBJTriangulatesPolygonAndIgnoresMetadata(t *testing.T) {
	mesh := loadOBJFixture(t, "quad.obj", strings.Join([]string{
		"o quad",
		"v 0 0 0",
		"v 1 0 0",
		"v 1 1 0",
		"v 0 1 0",
		"vt 0 0",
		"vn 0 0 1",
		"f 1/1/1 2/1/1 3/1/1 4/1/1",
	}, "\n"))

	if len(mesh.Triangles) != 2 {
		t.Fatalf("len(Triangles) = %d, want 2", len(mesh.Triangles))
	}
}

func TestLoadOBJSupportsNegativeIndices(t *testing.T) {
	mesh := loadOBJFixture(t, "negative.obj", strings.Join([]string{
		"v 0 0 0",
		"v 1 0 0",
		"v 0 1 0",
		"f -3 -2 -1",
	}, "\n"))

	if len(mesh.Triangles) != 1 {
		t.Fatalf("len(Triangles) = %d, want 1", len(mesh.Triangles))
	}
}

func TestLoadOBJRejectsOutOfRangeIndex(t *testing.T) {
	path := writeOBJFixture(t, "invalid.obj", strings.Join([]string{
		"v 0 0 0",
		"v 1 0 0",
		"v 0 1 0",
		"f 1 2 4",
	}, "\n"))

	_, err := geometry.LoadOBJ(path)
	if err == nil {
		t.Fatal("LoadOBJ() returned nil for out-of-range face index")
	}

	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("LoadOBJ() error = %q, want out-of-range message", err)
	}
}

func loadOBJFixture(t *testing.T, name, contents string) *geometry.Mesh {
	t.Helper()

	path := writeOBJFixture(t, name, contents)

	mesh, err := geometry.LoadOBJ(path)
	if err != nil {
		t.Fatalf("LoadOBJ() error = %v", err)
	}

	return mesh
}

func writeOBJFixture(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)

	err := os.WriteFile(path, []byte(contents), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
