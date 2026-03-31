package main

import (
	"fmt"
	"os"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/pde"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

func main() {
	sc := &scene.Scene{
		Room:       scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Width: 3, Depth: 2.5, Height: 2.2}},
		Sources:    []scene.Source{{Position: scene.Source{}.Position}},
		Receivers:  []scene.Receiver{{Position: scene.Receiver{}.Position}},
		SampleRate: 48000,
		BandSpec:   acoustics.Octave8,
	}
	sc.Sources[0].Position = scene.Shoebox{}.Bounds().Center()
	sc.Sources[0].Position.X = 1.1
	sc.Sources[0].Position.Y = 1.0
	sc.Sources[0].Position.Z = 1.0
	sc.Receivers[0].Position.X = 1.8
	sc.Receivers[0].Position.Y = 1.2
	sc.Receivers[0].Position.Z = 1.0

	tracer := raytrace.RayTracer{Config: raytrace.LaunchConfig{NumRays: 2048, MaxBounces: 6, MaxTimeSeconds: 1.0, SpeedOfSound: acoustics.SpeedOfSound}, Scene: sc, ReceiverRadius: 0.2, BinDurationSeconds: 0.01}

	hist, err := tracer.Trace()
	if err != nil {
		panic(err)
	}

	geo := hybrid.HistogramToBuffer(hist, sc.SampleRate)

	transfer, err := pde.SweepShoebox(sc.Room.Shoebox, sc.Sources[0].Position, sc.Receivers[0].Position, pde.SweepConfig{FreqMin: 20, FreqMax: 300, NumPoints: 32, BoundaryCondition: "neumann"})
	if err != nil {
		panic(err)
	}

	low := transfer.ToTimeDomain(sc.SampleRate, len(geo.Samples))

	combined := hybrid.BlendLowFreq(low, geo, 200, sc.SampleRate)
	if combined == nil {
		panic("combine failed")
	}

	if err := export.WriteMonoWAV("output.wav", combined); err != nil {
		panic(err)
	}

	fmt.Fprintln(os.Stdout, "wrote output.wav")
}
