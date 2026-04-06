package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

// TestShoeboxAndNearShoeboxMeshHaveCloseAcoustics verifies that a shoebox room
// and a mesh room with the same geometry (one corner vertex displaced by 1 mm)
// and the same surface material produce nearly identical acoustic metrics.
//
// This test exercises the full material-lookup path for mesh rooms and guards
// against regressions where mesh rooms silently fall back to fully-reflective
// walls, which would produce a radically different (much longer) T30.
func TestShoeboxAndNearShoeboxMeshHaveCloseAcoustics(t *testing.T) {
	t.Parallel()

	const (
		width  = 8.0
		depth  = 6.0
		height = 4.0
	)

	material := scene.Material{
		Name:             "plaster",
		AbsorptionByBand: []float64{0.15, 0.15, 0.20, 0.20, 0.25, 0.25},
		ScatteringByBand: []float64{0.05, 0.05, 0.05, 0.05, 0.05, 0.05},
	}
	materials := map[string]scene.Material{"plaster": material}
	src := []scene.Source{{
		Position:    geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.2},
		Orientation: geometry.QuatIdentity(),
		Directivity: directivity.OmniModel{},
	}}
	rcv := []scene.Receiver{{
		Position:    geometry.Vec3{X: 6.5, Y: 4.5, Z: 2.5},
		Orientation: geometry.QuatIdentity(),
		Type:        scene.ReceiverOmni,
	}}

	shoeboxSc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width: width, Depth: depth, Height: height,
				WallMaterials: [6]string{"plaster", "plaster", "plaster", "plaster", "plaster", "plaster"},
			},
		},
		Materials:  materials,
		Sources:    src,
		Receivers:  rcv,
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	// Near-shoebox mesh: same box with one corner vertex displaced by 1 mm.
	// The perturbation is acoustically negligible but forces the mesh code path.
	const delta = 0.001
	minCorner := geometry.Vec3{X: 0, Y: 0, Z: 0}
	maxCorner := geometry.Vec3{X: width, Y: depth, Z: height}
	mesh := nearShoeboxMesh(minCorner, maxCorner, delta)

	meshSc := &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "plaster",
		},
		Materials:  materials,
		Sources:    src,
		Receivers:  rcv,
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	cfg := LaunchConfig{
		NumRays:        50000,
		MaxBounces:     40,
		MaxTimeSeconds: 4.0,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}

	shoeboxMetrics := traceSceneMetrics(t, shoeboxSc, cfg, 0.5)
	meshMetrics := traceSceneMetrics(t, meshSc, cfg, 0.5)

	// T30: shoebox and near-shoebox mesh must agree within ±15 %.
	if shoeboxMetrics.t30 <= 0 || meshMetrics.t30 <= 0 {
		t.Fatalf("T30 must be positive: shoebox=%v mesh=%v", shoeboxMetrics.t30, meshMetrics.t30)
	}

	t30RelDiff := math.Abs(shoeboxMetrics.t30-meshMetrics.t30) / shoeboxMetrics.t30
	if t30RelDiff > 0.15 {
		t.Fatalf("T30 relative difference = %.3f (shoebox=%.3fs mesh=%.3fs): want ≤ 0.15",
			t30RelDiff, shoeboxMetrics.t30, meshMetrics.t30)
	}

	// C80: must agree within ±3 dB.
	c80Diff := math.Abs(shoeboxMetrics.c80 - meshMetrics.c80)
	if c80Diff > 3.0 {
		t.Fatalf("C80 absolute difference = %.2f dB (shoebox=%.2f mesh=%.2f): want ≤ 3 dB",
			c80Diff, shoeboxMetrics.c80, meshMetrics.c80)
	}

	// D50: must agree within ±0.10 (absolute).
	d50Diff := math.Abs(shoeboxMetrics.d50 - meshMetrics.d50)
	if d50Diff > 0.10 {
		t.Fatalf("D50 absolute difference = %.3f (shoebox=%.3f mesh=%.3f): want ≤ 0.10",
			d50Diff, shoeboxMetrics.d50, meshMetrics.d50)
	}
}

// TestMeshRoomWithMaterialHasShorterT30ThanFullyReflective verifies that a
// mesh room with absorptive surface material actually absorbs energy. Before
// the MeshMaterial fix, mesh rooms silently used fully-reflective walls, so
// T30(absorptive) was indistinguishable from T30(fully-reflective).
func TestMeshRoomWithMaterialHasShorterT30ThanFullyReflective(t *testing.T) {
	t.Parallel()

	mesh := cubeMesh(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 8, Y: 6, Z: 4},
	)

	baseCfg := scene.Scene{
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			Directivity: directivity.OmniModel{},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 6.5, Y: 4.5, Z: 2.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	// Fully-reflective mesh.
	reflSc := baseCfg
	reflSc.Room = scene.Room{Kind: scene.RoomKindMesh, Mesh: mesh}
	reflSc.Materials = map[string]scene.Material{}

	// Absorptive mesh (α=0.3).
	absSc := baseCfg
	absSc.Room = scene.Room{
		Kind: scene.RoomKindMesh, Mesh: mesh, MeshMaterial: "m",
	}
	absSc.Materials = map[string]scene.Material{
		"m": {
			Name:             "m",
			AbsorptionByBand: []float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3},
			ScatteringByBand: []float64{0.05, 0.05, 0.05, 0.05, 0.05, 0.05},
		},
	}

	cfg := LaunchConfig{
		NumRays:        50000,
		MaxBounces:     40,
		MaxTimeSeconds: 6.0,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}

	reflMetrics := traceSceneMetrics(t, &reflSc, cfg, 0.5)
	absMetrics := traceSceneMetrics(t, &absSc, cfg, 0.5)

	if reflMetrics.t30 <= absMetrics.t30 {
		t.Fatalf("T30(reflective)=%v must be greater than T30(absorptive)=%v: mesh material is not applied",
			reflMetrics.t30, absMetrics.t30)
	}
}

// nearShoeboxMesh builds a 12-triangle box mesh (identical to cubeMesh) with
// one corner vertex displaced by delta along the X axis. The perturbation is
// acoustically negligible but forces the general mesh code path instead of
// any axis-aligned box shortcut.
func nearShoeboxMesh(minCorner, maxCorner geometry.Vec3, delta float64) *geometry.Mesh {
	mesh := cubeMesh(minCorner, maxCorner)
	// Displace every occurrence of the min-corner vertex by delta along X.
	corner := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	displaced := geometry.Vec3{X: minCorner.X + delta, Y: minCorner.Y, Z: minCorner.Z}

	for i := range mesh.Triangles {
		if mesh.Triangles[i].V0 == corner {
			mesh.Triangles[i].V0 = displaced
		}

		if mesh.Triangles[i].V1 == corner {
			mesh.Triangles[i].V1 = displaced
		}

		if mesh.Triangles[i].V2 == corner {
			mesh.Triangles[i].V2 = displaced
		}
	}

	return mesh
}

// TestMeshMaterialMissingFallsBackToFullyReflective verifies that a mesh room
// with no MeshMaterial set (empty string) still traces without error and
// defaults to fully-reflective walls (the pre-existing safe default).
func TestMeshMaterialMissingFallsBackToFullyReflective(t *testing.T) {
	t.Parallel()

	mesh := cubeMesh(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 4, Y: 3, Z: 2.5},
	)
	sc := &scene.Scene{
		Room:      scene.Room{Kind: scene.RoomKindMesh, Mesh: mesh}, // no MeshMaterial
		Materials: map[string]scene.Material{},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1, Y: 1, Z: 1},
			Orientation: geometry.QuatIdentity(),
			Directivity: directivity.OmniModel{},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 3, Y: 2, Z: 2},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	tracer := &RayTracer{
		Config: LaunchConfig{
			NumRays:        1000,
			MaxBounces:     8,
			MaxTimeSeconds: 1.0,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:          sc,
		ReceiverRadius: 0.3,
	}

	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	buf := hist.ToLateMono(sc.SampleRate)
	if buf.Len() == 0 {
		t.Fatal("expected non-empty buffer for fully-reflective mesh room")
	}

	// Without absorption, the buffer must contain energy.
	var energy float64
	for _, s := range buf.Samples {
		energy += s * s
	}

	if energy <= 0 {
		t.Fatal("expected non-zero energy in buffer for fully-reflective mesh room")
	}
}

// TestNearShoeboxMeshT30IsFiniteAndPositive is a fast sanity check: the
// near-shoebox mesh must produce a finite positive T30.
func TestNearShoeboxMeshT30IsFiniteAndPositive(t *testing.T) {
	t.Parallel()

	const delta = 0.001
	mesh := nearShoeboxMesh(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 8, Y: 6, Z: 4},
		delta,
	)
	sc := &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "m",
		},
		Materials: map[string]scene.Material{
			"m": {
				Name:             "m",
				AbsorptionByBand: []float64{0.15, 0.15, 0.20, 0.20, 0.25, 0.25},
				ScatteringByBand: []float64{0.05, 0.05, 0.05, 0.05, 0.05, 0.05},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			Directivity: directivity.OmniModel{},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 6.5, Y: 4.5, Z: 2.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	tracer := &RayTracer{
		Config: LaunchConfig{
			NumRays:        20000,
			MaxBounces:     40,
			MaxTimeSeconds: 3.0,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:          sc,
		ReceiverRadius: 0.5,
	}

	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	buf := hist.ToLateMono(sc.SampleRate)

	t30, err := metrics.T30(buf)
	if err != nil {
		t.Fatalf("T30() error = %v", err)
	}

	if t30 <= 0 || math.IsNaN(t30) || math.IsInf(t30, 0) {
		t.Fatalf("T30 = %v, want positive finite", t30)
	}
}
