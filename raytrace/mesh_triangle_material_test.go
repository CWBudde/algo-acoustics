package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// triangleMaterialTraceScene builds a box-shaped mesh room whose whole-mesh
// material is absorbent. When overrideAll is set, the per-triangle table
// replaces every triangle with a hard material.
func triangleMaterialTraceScene(overrideAll bool) *scene.Scene {
	mesh := geometry.MeshFromBox(geometry.Vec3Zero, geometry.Vec3{X: 8, Y: 6, Z: 4})

	sc := &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "soft",
		},
		Materials: map[string]scene.Material{
			"soft": {Name: "soft", AbsorptionByBand: []float64{0.7}, ScatteringByBand: []float64{0.1}},
			"hard": {Name: "hard", AbsorptionByBand: []float64{0.02}, ScatteringByBand: []float64{0.1}},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 6, Y: 4, Z: 2},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	if !overrideAll {
		return sc
	}

	materials := make([]string, len(mesh.Triangles))
	for index := range materials {
		materials[index] = "hard"
	}

	sc.Room.TriangleMaterials = materials

	return sc
}

func totalHistogramEnergy(t *testing.T, sc *scene.Scene) float64 {
	t.Helper()

	tracer := &RayTracer{
		Config: LaunchConfig{
			NumRays:        4000,
			MaxBounces:     30,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:          sc,
		ReceiverRadius: 0.5,
	}

	histogram, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}

	total := 0.0

	for _, bin := range histogram.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
		}
	}

	return total
}

func TestMeshTriangleMaterialsChangeTracedEnergy(t *testing.T) {
	t.Parallel()

	softEnergy := totalHistogramEnergy(t, triangleMaterialTraceScene(false))
	hardEnergy := totalHistogramEnergy(t, triangleMaterialTraceScene(true))

	if softEnergy <= 0 || hardEnergy <= 0 {
		t.Fatalf("both traces must collect energy: soft=%v hard=%v", softEnergy, hardEnergy)
	}

	// Absorption drops from 0.7 to 0.02 on every surface, so the hard room must
	// retain far more energy. Before the per-triangle table was plumbed through
	// sceneMaterialForWall, both scenes resolved the same MeshMaterial and these
	// totals were identical.
	if hardEnergy <= softEnergy*2 {
		t.Fatalf("hard-walled energy = %v, soft-walled = %v: want the hard room to retain much more",
			hardEnergy, softEnergy)
	}
}

func TestSceneMeshTriangleMaterialsIsNilWithoutTable(t *testing.T) {
	t.Parallel()

	if got := sceneMeshTriangleMaterials(triangleMaterialTraceScene(false)); got != nil {
		t.Fatalf("sceneMeshTriangleMaterials = %v, want nil for a single-material room", got)
	}

	sc := triangleMaterialTraceScene(true)

	got := sceneMeshTriangleMaterials(sc)
	if len(got) != len(sc.Room.Mesh.Triangles) {
		t.Fatalf("sceneMeshTriangleMaterials length = %d, want %d", len(got), len(sc.Room.Mesh.Triangles))
	}

	if got[0].Name != "hard" {
		t.Fatalf("triangle 0 material = %q, want hard", got[0].Name)
	}
}

func TestSceneMaterialForWallResolvesMeshTrianglesIndividually(t *testing.T) {
	t.Parallel()

	sc := triangleMaterialTraceScene(false)

	// Override only the -X face, which MeshFromBox emits as triangles 0 and 1.
	materials := make([]string, len(sc.Room.Mesh.Triangles))
	materials[0] = "hard"
	materials[1] = "hard"
	sc.Room.TriangleMaterials = materials

	tracer := &RayTracer{Scene: sc}

	if got := tracer.sceneMaterialForWall(0).Name; got != "hard" {
		t.Fatalf("triangle 0 material = %q, want hard", got)
	}

	if got := tracer.sceneMaterialForWall(4).Name; got != "soft" {
		t.Fatalf("triangle 4 material = %q, want the soft fallback", got)
	}
}
