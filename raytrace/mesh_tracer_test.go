package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestNewMeshTracerAllowsWarningOnlyMesh(t *testing.T) {
	t.Parallel()

	mesh := &geometry.Mesh{Triangles: []geometry.Triangle{{
		V0: geometry.Vec3{X: 0, Y: 0, Z: 1},
		V1: geometry.Vec3{X: 1, Y: 0, Z: 1},
		V2: geometry.Vec3{X: 0, Y: 1, Z: 1},
	}}}

	tracer, err := NewMeshTracer(mesh, nil)
	if err != nil {
		t.Fatalf("NewMeshTracer() error = %v", err)
	}

	if tracer.BVH == nil {
		t.Fatal("NewMeshTracer() returned nil BVH")
	}
}

func TestNewMeshTracerRejectsDegenerateMesh(t *testing.T) {
	t.Parallel()

	mesh := &geometry.Mesh{Triangles: []geometry.Triangle{{
		V0: geometry.Vec3{X: 0, Y: 0, Z: 1},
		V1: geometry.Vec3{X: 1, Y: 0, Z: 1},
		V2: geometry.Vec3{X: 2, Y: 0, Z: 1},
	}}}

	_, err := NewMeshTracer(mesh, nil)
	if err == nil {
		t.Fatal("NewMeshTracer() error = nil, want validation failure")
	}
}

func TestMeshTracerNextHitFindsNearestTriangle(t *testing.T) {
	t.Parallel()

	mesh := &geometry.Mesh{Triangles: []geometry.Triangle{
		{
			V0: geometry.Vec3{X: -1, Y: -1, Z: 1},
			V1: geometry.Vec3{X: 0, Y: 1, Z: 1},
			V2: geometry.Vec3{X: 1, Y: -1, Z: 1},
		},
		{
			V0: geometry.Vec3{X: -1, Y: -1, Z: 3},
			V1: geometry.Vec3{X: 0, Y: 1, Z: 3},
			V2: geometry.Vec3{X: 1, Y: -1, Z: 3},
		},
	}}

	tracer, err := NewMeshTracer(mesh, []*scene.Material{nil, nil})
	if err != nil {
		t.Fatalf("NewMeshTracer() error = %v", err)
	}

	hitPoint, normal, wallIdx, ok := tracer.NextHit(geometry.NewRay(geometry.Vec3Zero, geometry.Vec3{Z: 1}))
	if !ok {
		t.Fatal("NextHit() returned ok=false")
	}

	if wallIdx != 0 {
		t.Fatalf("wallIdx = %d, want 0", wallIdx)
	}

	if diff := math.Abs(hitPoint.Z - 1); diff > 1e-12 {
		t.Fatalf("hitPoint.Z = %g, want 1", hitPoint.Z)
	}

	if hitPoint.X != 0 || hitPoint.Y != 0 {
		t.Fatalf("hitPoint = %#v, want origin-projected hit", hitPoint)
	}

	if normal != mesh.Triangles[0].Normal() {
		t.Fatalf("normal = %#v, want %#v", normal, mesh.Triangles[0].Normal())
	}
}
