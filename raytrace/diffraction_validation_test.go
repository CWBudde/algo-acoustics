package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRayTracerDiffractionBranchSmoke(t *testing.T) {
	t.Parallel()

	mesh := loadValidationMeshCubeScene(t).Room.Mesh
	index := NewDiffractionEdgeIndex(mesh)
	if index == nil {
		t.Fatal("NewDiffractionEdgeIndex() returned nil")
	}

	edges := geometry.ExtractDiffractionEdges(mesh)
	branches := spawnDiffractionBranches(
		RayState{Ray: geometry.NewRay(geometry.Vec3{X: -1, Y: -0.05, Z: 0.5}, geometry.Vec3{X: 1, Y: 0, Z: 0}), Energy: []float64{1, 1}},
		geometry.NewRay(geometry.Vec3{X: -1, Y: -0.05, Z: 0.5}, geometry.Vec3{X: 1, Y: 0, Z: 0}),
		geometry.Vec3{X: 2, Y: -0.05, Z: 0.5},
		3,
		edges,
		index,
		LaunchConfig{DiffractionAngularThreshold: 0.3, DiffractionConeSamples: 8},
		nil,
		1,
		[]float64{125, 500},
	)

	if len(branches) == 0 {
		t.Fatal("spawnDiffractionBranches() returned no branches")
	}

	for i, branch := range branches {
		if branch.Ray.Direction.Norm() == 0 || math.Abs(branch.Ray.Direction.Norm()-1) > 1e-12 {
			t.Fatalf("branch %d direction norm = %v, want 1", i, branch.Ray.Direction.Norm())
		}
	}
}

func BenchmarkDiffractionBranchSpawn(b *testing.B) {
	mesh := loadValidationMeshCubeScene(b).Room.Mesh
	index := NewDiffractionEdgeIndex(mesh)
	edges := geometry.ExtractDiffractionEdges(mesh)
	ray := geometry.NewRay(geometry.Vec3{X: -1, Y: -0.05, Z: 0.5}, geometry.Vec3{X: 1, Y: 0, Z: 0})
	state := RayState{Ray: ray, Energy: []float64{1, 1}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = spawnDiffractionBranches(
			state,
			ray,
			geometry.Vec3{X: 2, Y: -0.05, Z: 0.5},
			3,
			edges,
			index,
			LaunchConfig{DiffractionAngularThreshold: 0.3, DiffractionConeSamples: 8},
			nil,
			1,
			[]float64{125, 500},
		)
	}
}

func loadValidationMeshCubeScene(tb testing.TB) *scene.Scene {
	tb.Helper()

	path := "../testdata/rooms/mesh_cube.json"
	sc, err := scene.LoadSceneFile(path)
	if err != nil {
		tb.Fatalf("LoadSceneFile(%q) error = %v", path, err)
	}

	return sc
}
