package ism

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestDiffractionEventsForPathEvaluatesEachBand(t *testing.T) {
	source := scene.Source{Position: geometry.Vec3{X: 2, Y: 1, Z: 1}, GainDB: 0}
	receiver := scene.Receiver{Position: geometry.Vec3{X: -2, Y: 1.5, Z: 3}, Type: scene.ReceiverOmni}
	path := geometry.DiffractionPath{
		Source:   source.Position,
		Receiver: receiver.Position,
		Point:    geometry.Vec3{X: 0, Y: 0, Z: 2},
		Edge: geometry.DiffractionEdge{
			Start:      geometry.Vec3{X: 0, Y: 0, Z: 0},
			End:        geometry.Vec3{X: 0, Y: 0, Z: 4},
			Direction:  geometry.Vec3{Z: 1},
			Length:     4,
			WedgeIndex: 1.5,
			FaceONormal: geometry.Vec3{
				X: 1,
			},
		},
	}
	path.SourceDistance = source.Position.Distance(path.Point)
	path.ReceiverDistance = receiver.Position.Distance(path.Point)
	path.TotalDistance = path.SourceDistance + path.ReceiverDistance

	events := diffractionEventsForPath(source, receiver, path, acoustics.BandSpec{
		CenterFreqs: []float64{125, 1000},
		LowerEdges:  []float64{88, 707},
		UpperEdges:  []float64{177, 1414},
	}, acoustics.SpeedOfSound, diffractionCullingLevelDB)

	if len(events) != 2 {
		t.Fatalf("diffractionEventsForPath() len = %d, want 2", len(events))
	}

	for index, event := range events {
		if event.Kind != ir.EventDiffraction {
			t.Fatalf("event[%d].Kind = %v, want diffraction", index, event.Kind)
		}

		if event.Amplitude <= 0 {
			t.Fatalf("event[%d].Amplitude = %v, want positive", index, event.Amplitude)
		}

		if math.IsNaN(event.PhaseRadians) || math.IsInf(event.PhaseRadians, 0) {
			t.Fatalf("event[%d].PhaseRadians = %v, want finite", index, event.PhaseRadians)
		}
	}

	if math.Abs(events[0].TimeSeconds-events[1].TimeSeconds) > 1e-12 {
		t.Fatalf("event times differ, want one path time: %v vs %v", events[0].TimeSeconds, events[1].TimeSeconds)
	}

	if math.Abs(events[0].PhaseRadians-events[1].PhaseRadians) <= 1e-12 {
		t.Fatal("band-specific phase should differ across octave bands")
	}
}

func TestDiffractionEventsForPathCullsWeakContribution(t *testing.T) {
	source := scene.Source{Position: geometry.Vec3{X: 2, Y: 1, Z: 1}, GainDB: 0}
	receiver := scene.Receiver{Position: geometry.Vec3{X: -2, Y: 1.5, Z: 3}, Type: scene.ReceiverOmni}
	path := geometry.DiffractionPath{
		Source:   source.Position,
		Receiver: receiver.Position,
		Point:    geometry.Vec3{X: 0, Y: 0, Z: 2},
		Edge: geometry.DiffractionEdge{
			Start:      geometry.Vec3{X: 0, Y: 0, Z: 0},
			End:        geometry.Vec3{X: 0, Y: 0, Z: 4},
			Direction:  geometry.Vec3{Z: 1},
			Length:     4,
			WedgeIndex: 1.5,
			FaceONormal: geometry.Vec3{
				X: 1,
			},
		},
	}
	path.SourceDistance = source.Position.Distance(path.Point)
	path.ReceiverDistance = receiver.Position.Distance(path.Point)
	path.TotalDistance = path.SourceDistance + path.ReceiverDistance

	events := diffractionEventsForPath(source, receiver, path, acoustics.BandSpec{
		CenterFreqs: []float64{125},
		LowerEdges:  []float64{88},
		UpperEdges:  []float64{177},
	}, acoustics.SpeedOfSound, 1000)

	if len(events) != 0 {
		t.Fatalf("diffractionEventsForPath() returned %d events, want 0 after culling", len(events))
	}
}

func TestDiffractionEventsEnumeratesBarrierPath(t *testing.T) {
	mesh := barrierMesh()
	edges := geometry.ExtractDiffractionEdges(mesh)

	source := scene.Source{Position: geometry.Vec3{X: -6.5085842324344085, Y: 1.921311494612064, Z: 1.5096748432605978}, GainDB: 0}
	receiver := scene.Receiver{Position: geometry.Vec3{X: 4.20577077742135, Y: -1.6274612034653257, Z: 1.5365044111705466}, Type: scene.ReceiverOmni}

	events := DiffractionEvents(source, receiver, edges, mesh, acoustics.BandSpec{
		CenterFreqs: []float64{500},
		LowerEdges:  []float64{354},
		UpperEdges:  []float64{707},
	}, acoustics.SpeedOfSound)

	if len(events) == 0 {
		t.Fatal("DiffractionEvents() returned no events, want at least one")
	}

	for index, event := range events {
		if event.Kind != ir.EventDiffraction {
			t.Fatalf("event[%d].Kind = %v, want diffraction", index, event.Kind)
		}
	}
}

func barrierMesh() *geometry.Mesh {
	minCorner := geometry.Vec3{X: -0.1, Y: -1, Z: 0}
	maxCorner := geometry.Vec3{X: 0.1, Y: 1, Z: 2}
	v000 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v001 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v010 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v011 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}
	v100 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v101 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v110 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v111 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}

	return &geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: v000, V1: v110, V2: v100},
		{V0: v000, V1: v010, V2: v110},
		{V0: v001, V1: v101, V2: v111},
		{V0: v001, V1: v111, V2: v011},
		{V0: v000, V1: v101, V2: v001},
		{V0: v000, V1: v100, V2: v101},
		{V0: v010, V1: v011, V2: v111},
		{V0: v010, V1: v111, V2: v110},
		{V0: v000, V1: v001, V2: v011},
		{V0: v000, V1: v011, V2: v010},
		{V0: v100, V1: v110, V2: v111},
		{V0: v100, V1: v111, V2: v101},
	}}
}
