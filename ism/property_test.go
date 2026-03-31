package ism

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
)

func TestISMSolverSolveDirectAmplitudeHalvesWhenDistanceDoubles(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)
	sc.Sources[0].Directivity = directivity.OmniModel{}
	sc.Receivers[0].Position = geometry.Vec3{X: 4, Y: 2, Z: 3}

	nearEvents, err := solver.Solve(&sc, ISMConfig{MaxOrder: 0, SpeedOfSound: acoustics.SpeedOfSound, BandSpec: sc.BandSpec})
	if err != nil {
		t.Fatalf("Solve(near) error = %v", err)
	}

	nearDirect := firstEventOfKind(nearEvents, ir.EventDirect)
	if nearDirect == nil {
		t.Fatal("expected near direct event")
	}

	sc.Receivers[0].Position = geometry.Vec3{X: 7, Y: 2, Z: 3}

	farEvents, err := solver.Solve(&sc, ISMConfig{MaxOrder: 0, SpeedOfSound: acoustics.SpeedOfSound, BandSpec: sc.BandSpec})
	if err != nil {
		t.Fatalf("Solve(far) error = %v", err)
	}

	farDirect := firstEventOfKind(farEvents, ir.EventDirect)
	if farDirect == nil {
		t.Fatal("expected far direct event")
	}

	if math.Abs(farDirect.Amplitude-nearDirect.Amplitude/2) > 1e-12 {
		t.Fatalf("far amplitude = %v, want half of near amplitude %v", farDirect.Amplitude, nearDirect.Amplitude)
	}

	attenuationDB := 20 * math.Log10(farDirect.Amplitude/nearDirect.Amplitude)
	if math.Abs(attenuationDB+6.020599913279624) > 1e-9 {
		t.Fatalf("attenuation = %v dB, want approximately -6.02 dB", attenuationDB)
	}
}

func TestISMSolverSolveMonotonicDecayStrengthensWithAbsorption(t *testing.T) {
	t.Parallel()

	levels := []float64{0.2, 0.4, 0.6, 0.8}
	previousT60 := math.Inf(1)

	for _, level := range levels {
		buf := renderAbsorptionScene(t, level)

		currentT60, err := metrics.T60FromDecaySlope(buf)
		if err != nil {
			t.Fatalf("T60FromDecaySlope(level=%v) error = %v", level, err)
		}

		if currentT60 >= previousT60 {
			t.Fatalf("T60 = %v, want strictly below previous %v for higher absorption", currentT60, previousT60)
		}

		previousT60 = currentT60
	}
}

func renderAbsorptionScene(t *testing.T, absorption float64) *ir.Buffer {
	t.Helper()

	sc := testScene(t)
	for name, material := range sc.Materials {
		material.AbsorptionByBand = []float64{absorption, absorption, absorption, absorption, absorption, absorption}
		sc.Materials[name] = material
	}

	solver := ISMSolver{}

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 3, SpeedOfSound: acoustics.SpeedOfSound, BandSpec: sc.BandSpec})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	buf, err := ir.RenderMono(events, ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 2.5, BandSpec: sc.BandSpec})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	return buf
}
