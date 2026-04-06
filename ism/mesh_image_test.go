package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestGenerateMeshImageSourcesOrder0(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	bvh := geometry.BuildBVH(mesh)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	sources := GenerateMeshImageSources(src, mesh, bvh, MeshISMConfig{
		MaxOrder: 0,
	})

	if len(sources) != 1 {
		t.Fatalf("order 0: got %d sources, want 1", len(sources))
	}

	if sources[0].Position != src {
		t.Fatalf("order 0: position = %v, want %v", sources[0].Position, src)
	}

	if sources[0].Order != 0 {
		t.Fatalf("order 0: order = %d, want 0", sources[0].Order)
	}
}

func TestGenerateMeshImageSourcesOrder1(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	bvh := geometry.BuildBVH(mesh)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	sources := GenerateMeshImageSources(src, mesh, bvh, MeshISMConfig{
		MaxOrder: 1,
	})

	// Order 0: 1 source. Order 1: at least 6 (one per face plane).
	order1Count := 0

	for _, s := range sources {
		if s.Order == 1 {
			order1Count++
		}
	}

	if order1Count < 6 {
		t.Fatalf("order 1: got %d sources, want >= 6", order1Count)
	}

	// Total should be at least 7 (1 order-0 + 6 order-1).
	if len(sources) < 7 {
		t.Fatalf("total sources = %d, want >= 7", len(sources))
	}
}

func TestGenerateMeshImageSourcesRespectsMaxDistance(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	bvh := geometry.BuildBVH(mesh)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	far := GenerateMeshImageSources(src, mesh, bvh, MeshISMConfig{
		MaxOrder:    3,
		MaxDistance:  100,
	})

	near := GenerateMeshImageSources(src, mesh, bvh, MeshISMConfig{
		MaxOrder:    3,
		MaxDistance:  1.5,
	})

	if len(near) >= len(far) {
		t.Fatalf("short distance produced %d sources, long distance produced %d; expected fewer", len(near), len(far))
	}
}

func TestGenerateMeshImageSourcesRespectsMaxCandidates(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	bvh := geometry.BuildBVH(mesh)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	cap := 10

	sources := GenerateMeshImageSources(src, mesh, bvh, MeshISMConfig{
		MaxOrder:      5,
		MaxCandidates: cap,
	})

	if len(sources) > cap {
		t.Fatalf("got %d sources, want <= %d", len(sources), cap)
	}
}
