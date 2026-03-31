package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRayTracerTraceProducesLateEnergy(t *testing.T) {
	t.Parallel()

	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"reflective", "reflective", "reflective", "reflective", "reflective", "reflective",
				},
			},
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2}, GainDB: -12}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	tracer := RayTracer{
		Config: LaunchConfig{NumRays: 10000, MaxBounces: 8, MaxTimeSeconds: 1, SpeedOfSound: acoustics.SpeedOfSound},
		Scene:  sc,
	}
	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if len(hist.Bins) == 0 {
		t.Fatal("Trace() returned empty histogram")
	}

	var total float64
	for _, bin := range hist.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
		}
	}
	if total <= 0 {
		t.Fatal("Trace() produced zero total energy")
	}
}

func TestRayTracerTraceSupportsMeshRoom(t *testing.T) {
	t.Parallel()

	mesh := cubeMesh(geometry.Vec3Zero, geometry.Vec3{X: 4, Y: 4, Z: 4})
	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindMesh,
			Mesh: mesh,
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1, Z: 1}, GainDB: -12}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 1.8, Y: 1, Z: 1}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	tracer := RayTracer{
		Config:         LaunchConfig{NumRays: 4096, MaxBounces: 6, MaxTimeSeconds: 1, SpeedOfSound: acoustics.SpeedOfSound},
		Scene:          sc,
		ReceiverRadius: 0.5,
	}
	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if len(hist.Bins) == 0 {
		t.Fatal("Trace() returned empty histogram")
	}

	var total float64
	for _, bin := range hist.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
		}
	}
	if total <= 0 {
		t.Fatal("Trace() produced zero total energy for mesh room")
	}
}

func cubeMesh(min, max geometry.Vec3) *geometry.Mesh {
	v000 := geometry.Vec3{X: min.X, Y: min.Y, Z: min.Z}
	v001 := geometry.Vec3{X: min.X, Y: min.Y, Z: max.Z}
	v010 := geometry.Vec3{X: min.X, Y: max.Y, Z: min.Z}
	v011 := geometry.Vec3{X: min.X, Y: max.Y, Z: max.Z}
	v100 := geometry.Vec3{X: max.X, Y: min.Y, Z: min.Z}
	v101 := geometry.Vec3{X: max.X, Y: min.Y, Z: max.Z}
	v110 := geometry.Vec3{X: max.X, Y: max.Y, Z: min.Z}
	v111 := geometry.Vec3{X: max.X, Y: max.Y, Z: max.Z}

	return &geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: v000, V1: v010, V2: v001},
		{V0: v001, V1: v010, V2: v011},
		{V0: v100, V1: v101, V2: v110},
		{V0: v101, V1: v111, V2: v110},
		{V0: v000, V1: v001, V2: v100},
		{V0: v001, V1: v101, V2: v100},
		{V0: v010, V1: v110, V2: v011},
		{V0: v011, V1: v110, V2: v111},
		{V0: v000, V1: v100, V2: v010},
		{V0: v010, V1: v100, V2: v110},
		{V0: v001, V1: v011, V2: v101},
		{V0: v011, V1: v111, V2: v101},
	}}
}
