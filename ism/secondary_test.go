package ism

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestSolveSecondaryAppliesPressureDelayAndPhase(t *testing.T) {
	t.Parallel()

	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:         6,
				Depth:         4,
				Height:        3,
				WallMaterials: [6]string{"wall", "wall", "wall", "wall", "wall", "wall"},
			},
		},
		Materials:  map[string]scene.Material{"wall": scene.MaterialFullyReflective()},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 4, Y: 2, Z: 1.5}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
	emission := ir.PressureEmission{
		Position:     geometry.Vec3{X: 1, Y: 2, Z: 1.5},
		TimeSeconds:  0.1,
		BandPressure: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
		PhaseRadians: 0.25,
	}

	events, err := (ISMSolver{}).SolveSecondary(sc, ISMConfig{MaxOrder: 0}, []ir.PressureEmission{emission})
	if err != nil {
		t.Fatalf("SolveSecondary() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("SolveSecondary() event count = %d, want 1", len(events))
	}

	event := events[0]

	wantTime := emission.TimeSeconds + 3/acoustics.SpeedOfSound
	if math.Abs(event.TimeSeconds-wantTime) > 1e-12 || event.Kind != ir.EventTransmission {
		t.Fatalf("secondary event = %#v, want time %v and transmission kind", event, wantTime)
	}

	if event.PhaseRadians != emission.PhaseRadians {
		t.Fatalf("phase = %v, want %v", event.PhaseRadians, emission.PhaseRadians)
	}

	for index, gain := range event.BandGain {
		if gain != 0.5 {
			t.Fatalf("band gain[%d] = %v, want 0.5", index, gain)
		}
	}
}

func TestSolveSecondaryRejectsEnergyDomainInputShape(t *testing.T) {
	t.Parallel()

	sc := &scene.Scene{BandSpec: acoustics.Octave6}

	_, err := (ISMSolver{}).SolveSecondary(sc, ISMConfig{}, []ir.PressureEmission{{BandPressure: []float64{1}}})
	if err == nil {
		t.Fatal("SolveSecondary() error = nil, want band-count error")
	}
}
