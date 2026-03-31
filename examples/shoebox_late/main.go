package main

import (
	"fmt"
	"os"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
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
		Materials:  map[string]scene.Material{"reflective": scene.MaterialFullyReflective()},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2}, GainDB: -12}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	tracer := raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        10000,
			MaxBounces:     12,
			MaxTimeSeconds: 2,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene: sc,
	}
	hist, err := tracer.Trace()
	if err != nil {
		return err
	}

	fmt.Println("time_seconds,energy")
	for _, bin := range hist.Bins {
		var total float64
		for _, bandEnergy := range bin.BandEnergy {
			total += bandEnergy
		}
		fmt.Printf("%.3f,%.6f\n", bin.TimeSeconds, total)
	}

	return nil
}
