package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestDiffractionEdgeIndexFindsNearbyEdges(t *testing.T) {
	t.Parallel()

	sc := loadMeshCubeScene(t)
	index := NewDiffractionEdgeIndex(sc.Room.Mesh)
	if index == nil {
		t.Fatal("NewDiffractionEdgeIndex() returned nil")
	}

	candidates := index.Candidates(
		geometry.Vec3{X: -1, Y: -0.05, Z: 0.5},
		geometry.Vec3{X: 2, Y: -0.05, Z: 0.5},
		0.2,
	)
	if len(candidates) == 0 {
		t.Fatal("Candidates() returned no edges, want nearby cube edges")
	}
}

func TestSpawnDiffractionBranchesProducesConeSamples(t *testing.T) {
	t.Parallel()

	sc := loadMeshCubeScene(t)
	mesh := sc.Room.Mesh
	index := NewDiffractionEdgeIndex(mesh)
	edges := geometry.ExtractDiffractionEdges(mesh)
	if len(edges) == 0 {
		t.Fatal("ExtractDiffractionEdges() returned no edges")
	}

	ray := geometry.NewRay(geometry.Vec3{X: -1, Y: -0.05, Z: 0.5}, geometry.Vec3{X: 1, Y: 0, Z: 0})
	state := RayState{
		Ray:        ray,
		Energy:     []float64{1, 1},
		PathLength: 0,
		Bounces:    0,
	}

	branches := spawnDiffractionBranches(
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

	if len(branches) == 0 {
		t.Fatal("spawnDiffractionBranches() returned no branches, want at least one")
	}

	for i, branch := range branches {
		if branch.Ray.Direction.Norm() == 0 || math.Abs(branch.Ray.Direction.Norm()-1) > 1e-12 {
			t.Fatalf("branch %d direction norm = %v, want 1", i, branch.Ray.Direction.Norm())
		}

		if len(branch.Energy) != 2 {
			t.Fatalf("branch %d energy length = %d, want 2", i, len(branch.Energy))
		}

		if maxEnergy(branch.Energy) <= 0 {
			t.Fatalf("branch %d energy = %#v, want positive", i, branch.Energy)
		}
	}
}
