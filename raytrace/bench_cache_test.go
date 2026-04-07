package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func newBenchScene() *scene.Scene {
	abs := make([]float64, scene.NumBands)
	for i := range abs {
		abs[i] = 0.1
	}

	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  10,
				Depth:  8,
				Height: 3,
				WallMaterials: [6]string{
					"walls", "walls", "walls",
					"walls", "floor", "ceiling",
				},
			},
		},
		Materials: map[string]scene.Material{
			"walls":   {AbsorptionByBand: append([]float64{}, abs...)},
			"floor":   {AbsorptionByBand: append([]float64{}, abs...)},
			"ceiling": {AbsorptionByBand: append([]float64{}, abs...)},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func BenchmarkTrace_10kRays(b *testing.B) {
	sc := newBenchScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10_000,
			MaxBounces:     100,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	b.ResetTimer()

	for range b.N {
		_, err := rt.Trace()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTracePaths_10kRays(b *testing.B) {
	sc := newBenchScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10_000,
			MaxBounces:     100,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	b.ResetTimer()

	for range b.N {
		_, err := rt.TracePaths()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluatePaths_10kRays(b *testing.B) {
	sc := newBenchScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10_000,
			MaxBounces:     100,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for range b.N {
		_, err := rt.EvaluatePaths(cache)
		if err != nil {
			b.Fatal(err)
		}
	}
}
