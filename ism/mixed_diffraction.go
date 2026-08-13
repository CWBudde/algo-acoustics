package ism

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func meshDiffractionEvents(
	sc *scene.Scene,
	cfg ISMConfig,
	bvh *geometry.BVHNode,
	material scene.Material,
	bandSpec acoustics.BandSpec,
	speedOfSound float64,
) []ir.Event {
	edges := geometry.ExtractDiffractionEdges(sc.Room.Mesh)
	if len(edges) == 0 {
		return nil
	}

	order := cfg.MaxDiffractionOrder
	if order == 0 {
		order = 1
	}

	receiver := sc.Receivers[0]
	events := make([]ir.Event, 0)

	for _, source := range sc.Sources {
		// A diffraction source is active only in the shadow of the
		// corresponding undiffracted source.
		if !geometry.SegmentVisible(sc.Room.Mesh, source.Position, receiver.Position) {
			events = append(events, DiffractionEvents(source, receiver, edges, sc.Room.Mesh, bandSpec, speedOfSound)...)
			if order >= 2 {
				paths := geometry.EnumerateSecondOrderDiffractionPaths(source.Position, receiver.Position, edges, sc.Room.Mesh)
				events = append(events, secondOrderDiffractionEvents(source, receiver, paths, bandSpec, speedOfSound)...)
			}
		}

		if order >= 2 && cfg.MaxOrder >= 1 {
			events = append(events, reflectionDiffractionEvents(source, receiver, edges, sc.Room.Mesh, bvh, material, bandSpec, speedOfSound)...)
			events = append(events, diffractionReflectionEvents(source, receiver, edges, sc.Room.Mesh, bvh, material, bandSpec, speedOfSound)...)
		}
	}

	return events
}

type mixedDiffractionPath struct {
	edge                geometry.DiffractionEdge
	diffractionSource   geometry.Vec3
	diffractionReceiver geometry.Vec3
	directivityTarget   geometry.Vec3
	arrivalDirection    geometry.Vec3
	totalDistance       float64
	extraDistance       float64
	reflectance         []float64
}

func reflectionDiffractionEvents(
	source scene.Source,
	receiver scene.Receiver,
	edges []geometry.DiffractionEdge,
	mesh *geometry.Mesh,
	bvh *geometry.BVHNode,
	material scene.Material,
	bandSpec acoustics.BandSpec,
	speedOfSound float64,
) []ir.Event {
	ppm := geometry.BuildPlanePolygonMap(mesh)
	images := GenerateMeshImageSources(source.Position, mesh, MeshISMConfig{MaxOrder: 1, PPM: ppm})
	paths := make([]mixedDiffractionPath, 0)

	for _, image := range images {
		if image.Order != 1 {
			continue
		}

		if _, visible := meshReflectionPath(image, receiver.Position, mesh, bvh, ppm); visible {
			continue
		}

		for _, candidate := range geometry.EnumerateDiffractionPaths(image.Position, receiver.Position, edges, nil) {
			reflectionPath, ok := meshReflectionPath(image, candidate.Point, mesh, bvh, ppm)
			if !ok || len(reflectionPath) != 1 || !geometry.SegmentVisible(mesh, source.Position, reflectionPath[0].Point) ||
				!geometry.SegmentVisible(mesh, candidate.Point, receiver.Position) {
				continue
			}

			reflection := reflectionPath[0]
			paths = append(paths, mixedDiffractionPath{
				edge:                candidate.Edge,
				diffractionSource:   reflection.Point,
				diffractionReceiver: receiver.Position,
				directivityTarget:   reflection.Point,
				arrivalDirection:    candidate.Point.Sub(receiver.Position).Normalize(),
				totalDistance:       source.Position.Distance(reflection.Point) + reflection.Point.Distance(candidate.Point) + candidate.Point.Distance(receiver.Position),
				extraDistance:       source.Position.Distance(reflection.Point),
				reflectance:         mixedReflectance([]meshReflectionPoint{reflection}, source.Position, material, bandSpec.BandCount()),
			})
		}
	}

	return mixedDiffractionPathEvents(source, receiver, paths, bandSpec, speedOfSound)
}

func diffractionReflectionEvents(
	source scene.Source,
	receiver scene.Receiver,
	edges []geometry.DiffractionEdge,
	mesh *geometry.Mesh,
	bvh *geometry.BVHNode,
	material scene.Material,
	bandSpec acoustics.BandSpec,
	speedOfSound float64,
) []ir.Event {
	ppm := geometry.BuildPlanePolygonMap(mesh)
	images := GenerateMeshImageSources(receiver.Position, mesh, MeshISMConfig{MaxOrder: 1, PPM: ppm})
	paths := make([]mixedDiffractionPath, 0)

	for _, image := range images {
		if image.Order != 1 {
			continue
		}

		if _, visible := meshReflectionPath(image, source.Position, mesh, bvh, ppm); visible {
			continue
		}

		for _, candidate := range geometry.EnumerateDiffractionPaths(source.Position, image.Position, edges, nil) {
			reflectionPath, ok := meshReflectionPath(image, candidate.Point, mesh, bvh, ppm)
			if !ok || len(reflectionPath) != 1 || !geometry.SegmentVisible(mesh, source.Position, candidate.Point) ||
				!geometry.SegmentVisible(mesh, reflectionPath[0].Point, receiver.Position) {
				continue
			}

			reflection := reflectionPath[0]
			paths = append(paths, mixedDiffractionPath{
				edge:                candidate.Edge,
				diffractionSource:   source.Position,
				diffractionReceiver: candidate.Point,
				directivityTarget:   candidate.Point,
				arrivalDirection:    reflection.Point.Sub(receiver.Position).Normalize(),
				totalDistance:       source.Position.Distance(candidate.Point) + candidate.Point.Distance(reflection.Point) + reflection.Point.Distance(receiver.Position),
				extraDistance:       reflection.Point.Distance(receiver.Position),
				reflectance:         mixedReflectance([]meshReflectionPoint{reflection}, candidate.Point, material, bandSpec.BandCount()),
			})
		}
	}

	return mixedDiffractionPathEvents(source, receiver, paths, bandSpec, speedOfSound)
}

func mixedReflectance(path []meshReflectionPoint, source geometry.Vec3, material scene.Material, bandCount int) []float64 {
	reflectance := make([]float64, bandCount)
	for bandIndex := range bandCount {
		reflectance[bandIndex] = meshPathReflectance(path, source, material, bandIndex)
	}

	return reflectance
}

func mixedDiffractionPathEvents(source scene.Source, receiver scene.Receiver, paths []mixedDiffractionPath, bandSpec acoustics.BandSpec, speedOfSound float64) []ir.Event {
	bandCount := bandSpec.BandCount()
	sourceLevel := sourceAmplitude(source)
	directDistance := source.Position.Distance(receiver.Position)

	if bandCount == 0 || sourceLevel <= 0 || directDistance <= pathEpsilon {
		return nil
	}

	minimumRelative := math.Pow(10, diffractionCullingLevelDB/20)
	events := make([]ir.Event, 0, len(paths)*bandCount)

	for _, path := range paths {
		launchGain := directivityBandGain(source, bandSpec, path.directivityTarget)
		for bandIndex, centerFrequency := range bandSpec.CenterFreqs {
			if bandIndex >= len(launchGain) || bandIndex >= len(path.reflectance) || launchGain[bandIndex] <= 0 || path.reflectance[bandIndex] <= 0 {
				continue
			}

			transfer, err := geometry.BTMETransfer(path.diffractionSource, path.diffractionReceiver, path.edge, centerFrequency, speedOfSound)
			if err != nil {
				continue
			}

			amplitude := sourceLevel * launchGain[bandIndex] * path.reflectance[bandIndex] * cmplx.Abs(transfer) / path.extraDistance
			if amplitude <= 0 || math.IsNaN(amplitude) || math.IsInf(amplitude, 0) || amplitude/(sourceLevel/directDistance) < minimumRelative {
				continue
			}

			bandGain := make([]float64, bandCount)
			bandGain[bandIndex] = 1
			phase := cmplx.Phase(transfer) - 2*math.Pi*centerFrequency*path.extraDistance/speedOfSound
			events = append(events, ir.Event{
				TimeSeconds:    path.totalDistance / speedOfSound,
				Amplitude:      amplitude,
				Direction:      path.arrivalDirection,
				DistanceMeters: path.totalDistance,
				BandGain:       bandGain,
				PhaseRadians:   phase,
				Kind:           ir.EventDiffraction,
			})
		}
	}

	return events
}
