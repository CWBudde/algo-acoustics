package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// triangleMaterialMeshScene builds a box-shaped mesh room. Every triangle keeps
// the absorbent whole-mesh material unless overrideNegX is set, in which case
// the two triangles of the -X face are overridden with a hard material through
// the per-triangle table.
func triangleMaterialMeshScene(t *testing.T, overrideNegX bool) *scene.Scene {
	t.Helper()

	mesh := geometry.MeshFromBox(geometry.Vec3Zero, geometry.Vec3{X: 6, Y: 4, Z: 3})

	sc := &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "soft",
		},
		Materials: map[string]scene.Material{
			"soft": {Name: "soft", AbsorptionByBand: []float64{0.8}, ScatteringByBand: []float64{0}},
			"hard": {Name: "hard", AbsorptionByBand: []float64{0.01}, ScatteringByBand: []float64{0}},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 3, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	if !overrideNegX {
		return sc
	}

	// MeshFromBox emits faces in the order -X, +X, -Y, +Y, -Z, +Z with two
	// triangles each, so the -X face is triangles 0 and 1.
	materials := make([]string, len(mesh.Triangles))
	materials[0] = "hard"
	materials[1] = "hard"
	sc.Room.TriangleMaterials = materials

	return sc
}

// firstOrderReflectionOffNegX finds the specular event whose path length matches
// a single bounce off the -X wall, which is the face the test overrides.
func firstOrderReflectionOffNegX(t *testing.T, sc *scene.Scene) (float64, bool) {
	t.Helper()

	events, err := (ISMSolver{}).Solve(sc, ISMConfig{MaxOrder: 1, BandSpec: acoustics.Octave6})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	// Mirror the source across x = 0 and match the resulting distance.
	mirrored := geometry.Vec3{X: -sc.Sources[0].Position.X, Y: sc.Sources[0].Position.Y, Z: sc.Sources[0].Position.Z}
	want := mirrored.Distance(sc.Receivers[0].Position)

	for _, event := range events {
		if event.Kind != ir.EventSpecular {
			continue
		}

		if diff := event.DistanceMeters - want; diff < 1e-6 && diff > -1e-6 {
			return event.BandGain[0], true
		}
	}

	return 0, false
}

func TestMeshTriangleMaterialsChangeSpecularBandGain(t *testing.T) {
	t.Parallel()

	uniformGain, ok := firstOrderReflectionOffNegX(t, triangleMaterialMeshScene(t, false))
	if !ok {
		t.Fatal("no first-order reflection off the -X wall in the uniform scene")
	}

	overriddenGain, ok := firstOrderReflectionOffNegX(t, triangleMaterialMeshScene(t, true))
	if !ok {
		t.Fatal("no first-order reflection off the -X wall in the overridden scene")
	}

	// Absorption drops from 0.8 to 0.01 on that wall, so the reflection must
	// come back markedly louder. Without per-triangle plumbing both scenes
	// would collapse onto the single MeshMaterial and these would be equal.
	if overriddenGain <= uniformGain*1.5 {
		t.Fatalf("overridden band gain = %v, uniform = %v: want the hard wall to reflect markedly more",
			overriddenGain, uniformGain)
	}
}

func TestMeshMaterialsFallsBackWithoutTriangleTable(t *testing.T) {
	t.Parallel()

	sc := triangleMaterialMeshScene(t, false)

	set := meshMaterials(sc)
	if set.perTriangle != nil {
		t.Fatal("a room without a triangle table must not allocate a per-triangle slice")
	}

	if got := set.At(0).Name; got != "soft" {
		t.Fatalf("At(0) = %q, want soft", got)
	}

	if got := set.At(9999).Name; got != "soft" {
		t.Fatalf("out-of-range At = %q, want the whole-mesh fallback", got)
	}
}

func TestMeshMaterialsUsesTriangleTableWhenPresent(t *testing.T) {
	t.Parallel()

	set := meshMaterials(triangleMaterialMeshScene(t, true))

	if got := set.At(0).Name; got != "hard" {
		t.Fatalf("At(0) = %q, want hard", got)
	}

	if got := set.At(4).Name; got != "soft" {
		t.Fatalf("At(4) = %q, want the soft fallback for an empty entry", got)
	}
}
