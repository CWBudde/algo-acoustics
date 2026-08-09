package raytrace

import (
	"reflect"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestSecondaryLaunchUsesFixedBudgetAndMixesDirections(t *testing.T) {
	t.Parallel()

	left := geometry.Vec3{X: 1, Y: 1, Z: 1}
	right := geometry.Vec3{X: 5, Y: 1, Z: 1}
	emissions := []ir.EnergyEmission{
		{Position: left, BandEnergy: []float64{1}},
		{Position: right, TimeSeconds: 0.01, BandEnergy: []float64{1}},
	}
	config := LaunchConfig{NumRays: 64, MaxTimeSeconds: 1}

	states := secondaryLaunchStates(emissions, config, geometry.Vec3{X: 3, Y: 2, Z: 1}, 0.25, 1)
	if len(states) != config.NumRays {
		t.Fatalf("secondary launch state count = %d, want fixed budget %d", len(states), config.NumRays)
	}

	type coverage struct {
		count                int
		positiveZ, negativeZ bool
	}

	byPosition := map[geometry.Vec3]*coverage{left: {}, right: {}}
	for _, state := range states {
		entry := byPosition[state.Ray.Origin]
		if entry == nil {
			t.Fatalf("unexpected launch origin %v", state.Ray.Origin)
		}

		entry.count++
		entry.positiveZ = entry.positiveZ || state.Ray.Direction.Z > 0
		entry.negativeZ = entry.negativeZ || state.Ray.Direction.Z < 0
	}

	for position, got := range byPosition {
		if got.count < 28 || got.count > 36 {
			t.Fatalf("origin %v ray count = %d, want balanced selection", position, got.count)
		}

		if !got.positiveZ || !got.negativeZ {
			t.Fatalf("origin %v direction coverage = %#v, want both hemispheres", position, got)
		}
	}
}

func TestTraceSecondaryIsDelayedDeterministicAndDirectional(t *testing.T) {
	t.Parallel()

	sc := secondaryRaytraceScene()
	config := LaunchConfig{
		NumRays:        4096,
		MaxBounces:     0,
		MaxTimeSeconds: 0.15,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}
	emissions := []ir.EnergyEmission{{
		Position:    geometry.Vec3{X: 1, Y: 2, Z: 1.5},
		TimeSeconds: 0.05,
		BandEnergy:  []float64{1, 1, 1, 1, 1, 1},
	}}

	trace := func() (*EnergyHistogram, []DirectivityGroup) {
		tracer := &RayTracer{
			Config:             config,
			Scene:              sc,
			ReceiverRadius:     0.5,
			BinDurationSeconds: 0.01,
			DirectivityGroups:  NewDirectivityGroups(4, 2),
		}

		histogram, err := tracer.TraceSecondary(emissions)
		if err != nil {
			t.Fatalf("TraceSecondary() error = %v", err)
		}

		return histogram, tracer.DirectivityGroups
	}

	first, firstGroups := trace()
	second, secondGroups := trace()

	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(firstGroups, secondGroups) {
		t.Fatal("TraceSecondary() is not deterministic")
	}

	var total, directionalTotal float64

	for _, bin := range first.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
			if bin.TimeSeconds < emissions[0].TimeSeconds && energy != 0 {
				t.Fatalf("energy arrived before emission delay: bin=%v energy=%v", bin.TimeSeconds, energy)
			}
		}
	}

	for _, group := range firstGroups {
		for _, bin := range group.Histogram.Bins {
			for _, energy := range bin.BandEnergy {
				directionalTotal += energy
			}
		}
	}

	if total <= 0 || directionalTotal <= 0 {
		t.Fatalf("secondary totals = main %v directional %v, want positive", total, directionalTotal)
	}
}

func secondaryRaytraceScene() *scene.Scene {
	return &scene.Scene{
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
}
