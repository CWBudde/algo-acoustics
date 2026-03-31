package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
)

const outputFilename = "output.wav"

func main() {
	err := run(outputFilename)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(outputPath string) error {
	if outputPath == "" {
		return errors.New("output path must not be empty")
	}

	sc := shoeboxScene()

	err := scene.Validate(sc)
	if err != nil {
		return err
	}

	solver := ism.ISMSolver{}

	events, err := solver.Solve(sc, ism.ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		return err
	}

	buffer, err := ir.RenderMono(events, ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: 1,
		BandSpec:        sc.BandSpec,
	})
	if err != nil {
		return err
	}

	err = export.WriteMonoWAV(outputPath, buffer)
	if err != nil {
		return err
	}

	return nil
}

func shoeboxScene() *scene.Scene {
	const roomWidth = 6.0
	const roomDepth = 4.5
	const roomHeight = 2.8

	bandSpec := acoustics.Octave6
	absorber := scene.Material{
		Name:             "plaster",
		AbsorptionByBand: []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
		ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
	}

	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  roomWidth,
				Depth:  roomDepth,
				Height: roomHeight,
				WallMaterials: [6]string{
					"plaster",
					"plaster",
					"plaster",
					"plaster",
					"plaster",
					"plaster",
				},
			},
		},
		Materials: map[string]scene.Material{
			"plaster": absorber,
		},
		Sources: []scene.Source{
			{
				Position:    geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2},
				Orientation: geometry.QuatIdentity(),
				GainDB:      -12,
				Directivity: directivity.OmniModel{},
			},
		},
		Receivers: []scene.Receiver{
			{
				Position:    geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2},
				Orientation: geometry.QuatIdentity(),
				Type:        scene.ReceiverOmni,
			},
		},
		BandSpec:   bandSpec,
		SampleRate: 48000,
	}
}
