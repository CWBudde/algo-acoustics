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
