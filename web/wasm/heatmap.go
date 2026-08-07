//go:build js && wasm

package main

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	heatmapColumns = 7
	heatmapRows    = 5
	heatmapFloorDB = -30.0
	heatmapInset   = 0.03
)

type demoSPLHeatmap struct {
	MinimumDB float64
	MaximumDB float64
	Samples   []demoSPLSample
}

type demoSPLSample struct {
	SurfaceID string
	X         float64
	Y         float64
	Z         float64
	LevelDB   float64
}

type heatmapProbe struct {
	surfaceID string
	display   geometry.Vec3
	receiver  geometry.Vec3
}

// buildDemoSPLHeatmap samples the broadband early field at the room boundary.
// Values are incoherent energy sums from the direct path and first-order ISM
// reflections, normalized to the strongest surface probe. The relative scale
// is deliberate: demo source amplitudes are normalized transfer gains rather
// than calibrated sound-power levels.
func buildDemoSPLHeatmap(sc *scene.Scene) (demoSPLHeatmap, error) {
	if sc == nil {
		return demoSPLHeatmap{}, errors.New("build SPL heatmap: scene is nil")
	}

	probes := heatmapProbes(sc)
	if len(probes) == 0 {
		return demoSPLHeatmap{}, errors.New("build SPL heatmap: room has no surfaces")
	}

	solver := &ism.CachedISMSolver{}
	cfg := ism.ISMConfig{
		MaxOrder:     1,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	}
	probeScene := *sc
	probeScene.Receivers = append([]scene.Receiver(nil), sc.Receivers...)
	energies := make([]float64, len(probes))
	maximumEnergy := 0.0

	for index, probe := range probes {
		if demoCancelled() {
			return demoSPLHeatmap{}, errors.New("render cancelled")
		}

		probeScene.Receivers[0].Position = probe.receiver
		events, err := solver.Solve(&probeScene, cfg)
		if err != nil {
			return demoSPLHeatmap{}, err
		}

		energy := broadbandEventEnergy(events)
		energies[index] = energy
		maximumEnergy = math.Max(maximumEnergy, energy)
	}

	if maximumEnergy <= 0 {
		return demoSPLHeatmap{}, errors.New("build SPL heatmap: simulated field is silent")
	}

	samples := make([]demoSPLSample, len(probes))
	minimumDB := 0.0
	for index, probe := range probes {
		levelDB := heatmapFloorDB
		if energies[index] > 0 {
			levelDB = math.Max(heatmapFloorDB, 10*math.Log10(energies[index]/maximumEnergy))
		}
		minimumDB = math.Min(minimumDB, levelDB)
		samples[index] = demoSPLSample{
			SurfaceID: probe.surfaceID,
			X:         probe.display.X,
			Y:         probe.display.Y,
			Z:         probe.display.Z,
			LevelDB:   levelDB,
		}
	}

	return demoSPLHeatmap{
		MinimumDB: minimumDB,
		MaximumDB: 0,
		Samples:   samples,
	}, nil
}

func heatmapProbes(sc *scene.Scene) []heatmapProbe {
	if sc.Room.Kind == scene.RoomKindMesh && sc.Room.Mesh != nil {
		return meshHeatmapProbes(sc.Room.Mesh, sc.Sources[0].Position)
	}
	if sc.Room.Shoebox == nil {
		return nil
	}

	room := sc.Room.Shoebox
	probes := make([]heatmapProbe, 0, 6*heatmapColumns*heatmapRows)
	probes = append(probes, gridHeatmapProbes("west", heatmapRows, heatmapColumns, func(row, column float64) heatmapProbe {
		y := column * room.Depth
		z := row * room.Height
		return heatmapProbe{display: geometry.Vec3{Y: y, Z: z}, receiver: geometry.Vec3{X: heatmapInset, Y: y, Z: z}}
	})...)
	probes = append(probes, gridHeatmapProbes("east", heatmapRows, heatmapColumns, func(row, column float64) heatmapProbe {
		y := column * room.Depth
		z := row * room.Height
		return heatmapProbe{display: geometry.Vec3{X: room.Width, Y: y, Z: z}, receiver: geometry.Vec3{X: room.Width - heatmapInset, Y: y, Z: z}}
	})...)
	probes = append(probes, gridHeatmapProbes("south", heatmapRows, heatmapColumns, func(row, column float64) heatmapProbe {
		x := column * room.Width
		z := row * room.Height
		return heatmapProbe{display: geometry.Vec3{X: x, Z: z}, receiver: geometry.Vec3{X: x, Y: heatmapInset, Z: z}}
	})...)
	probes = append(probes, gridHeatmapProbes("north", heatmapRows, heatmapColumns, func(row, column float64) heatmapProbe {
		x := column * room.Width
		z := row * room.Height
		return heatmapProbe{display: geometry.Vec3{X: x, Y: room.Depth, Z: z}, receiver: geometry.Vec3{X: x, Y: room.Depth - heatmapInset, Z: z}}
	})...)
	probes = append(probes, gridHeatmapProbes("floor", heatmapRows, heatmapColumns, func(row, column float64) heatmapProbe {
		x := column * room.Width
		y := row * room.Depth
		return heatmapProbe{display: geometry.Vec3{X: x, Y: y}, receiver: geometry.Vec3{X: x, Y: y, Z: heatmapInset}}
	})...)
	probes = append(probes, gridHeatmapProbes("ceiling", heatmapRows, heatmapColumns, func(row, column float64) heatmapProbe {
		x := column * room.Width
		y := row * room.Depth
		return heatmapProbe{display: geometry.Vec3{X: x, Y: y, Z: room.Height}, receiver: geometry.Vec3{X: x, Y: y, Z: room.Height - heatmapInset}}
	})...)

	return probes
}

func gridHeatmapProbes(surfaceID string, rows, columns int, build func(row, column float64) heatmapProbe) []heatmapProbe {
	probes := make([]heatmapProbe, 0, rows*columns)
	for row := range rows {
		rowFraction := float64(row) / float64(rows-1)
		for column := range columns {
			columnFraction := float64(column) / float64(columns-1)
			probe := build(rowFraction, columnFraction)
			probe.surfaceID = surfaceID
			probes = append(probes, probe)
		}
	}

	return probes
}

func meshHeatmapProbes(mesh *geometry.Mesh, source geometry.Vec3) []heatmapProbe {
	seen := make(map[geometry.Vec3]struct{})
	probes := make([]heatmapProbe, 0, len(mesh.Triangles)*3)
	for _, triangle := range mesh.Triangles {
		for _, vertex := range [...]geometry.Vec3{triangle.V0, triangle.V1, triangle.V2} {
			if _, ok := seen[vertex]; ok {
				continue
			}
			seen[vertex] = struct{}{}

			towardSource := source.Sub(vertex).Normalize()
			probes = append(probes, heatmapProbe{
				surfaceID: "mesh",
				display:   vertex,
				receiver:  vertex.Add(towardSource.Scale(heatmapInset)),
			})
		}
	}

	return probes
}

func broadbandEventEnergy(events []ir.Event) float64 {
	energy := 0.0
	for _, event := range events {
		bandEnergy := 1.0
		if len(event.BandGain) > 0 {
			bandEnergy = 0
			for _, gain := range event.BandGain {
				bandEnergy += gain * gain
			}
			bandEnergy /= float64(len(event.BandGain))
		}
		energy += event.Amplitude * event.Amplitude * bandEnergy
	}

	return energy
}
