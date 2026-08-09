package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestEventKindOrdering(t *testing.T) {
	tests := []struct {
		name string
		kind EventKind
		want int
	}{
		{name: "direct", kind: EventDirect, want: 0},
		{name: "specular", kind: EventSpecular, want: 1},
		{name: "diffuse", kind: EventDiffuse, want: 2},
		{name: "diffraction", kind: EventDiffraction, want: 3},
		{name: "pde", kind: EventPDE, want: 4},
		{name: "transmission", kind: EventTransmission, want: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := int(tc.kind); got != tc.want {
				t.Fatalf("kind = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEnergyEmissionToPressure(t *testing.T) {
	t.Parallel()

	emission := EnergyEmission{
		Position:    geometry.Vec3{X: 1, Y: 2, Z: 3},
		TimeSeconds: 0.25,
		BandEnergy:  []float64{1, 0.25, 0},
	}

	pressure := emission.ToPressure()
	if pressure.Position != emission.Position || pressure.TimeSeconds != emission.TimeSeconds {
		t.Fatalf("ToPressure() metadata = %#v, want position/time preserved", pressure)
	}

	want := []float64{1, 0.5, 0}
	for index := range want {
		if math.Abs(pressure.BandPressure[index]-want[index]) > 1e-12 {
			t.Fatalf("band pressure[%d] = %v, want %v", index, pressure.BandPressure[index], want[index])
		}
	}
}
