package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func phaseTestBandSpec() acoustics.BandSpec {
	return acoustics.BandSpec{
		CenterFreqs: []float64{500},
		LowerEdges:  []float64{350},
		UpperEdges:  []float64{700},
	}
}

func phaseTestRenderConfig() RenderConfig {
	return RenderConfig{SampleRate: 8000, DurationSeconds: 0.01, BandSpec: phaseTestBandSpec()}
}

// TestBandedFromEventsAllocatesQuadratureOnlyForPhasedEvents pins that the
// phase-free case, which is everything but ISM diffraction, stays on the cheap
// real-only representation.
func TestBandedFromEventsAllocatesQuadratureOnlyForPhasedEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		phase float64
		want  bool
	}{
		{name: "no phase", phase: 0, want: false},
		{name: "inverted", phase: math.Pi, want: false},
		{name: "quadrature", phase: math.Pi / 2, want: true},
		{name: "arbitrary", phase: 0.7, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response, err := BandedFromEvents(
				[]Event{{TimeSeconds: 0, Amplitude: 1, PhaseRadians: test.phase}},
				phaseTestRenderConfig(),
			)
			if err != nil {
				t.Fatalf("BandedFromEvents: %v", err)
			}

			if got := response.HasQuadrature(); got != test.want {
				t.Fatalf("HasQuadrature = %v, want %v", got, test.want)
			}
		})
	}
}

// TestBandedFromEventsSplitsAmplitudeIntoQuadrature pins that a phased event
// keeps its full magnitude across the two components instead of being projected
// onto the real axis and losing it.
func TestBandedFromEventsSplitsAmplitudeIntoQuadrature(t *testing.T) {
	t.Parallel()

	const phase = math.Pi / 3

	response, err := BandedFromEvents(
		[]Event{{TimeSeconds: 0, Amplitude: 2, PhaseRadians: phase}},
		phaseTestRenderConfig(),
	)
	if err != nil {
		t.Fatalf("BandedFromEvents: %v", err)
	}

	inPhase, quadrature := response.Bands[0][0], response.Quadrature[0][0]

	if diff := math.Abs(inPhase - 2*math.Cos(phase)); diff > 1e-12 {
		t.Fatalf("in-phase component = %v, want %v", inPhase, 2*math.Cos(phase))
	}

	if diff := math.Abs(quadrature - 2*math.Sin(phase)); diff > 1e-12 {
		t.Fatalf("quadrature component = %v, want %v", quadrature, 2*math.Sin(phase))
	}

	if magnitude := math.Hypot(inPhase, quadrature); math.Abs(magnitude-2) > 1e-12 {
		t.Fatalf("magnitude = %v, want 2", magnitude)
	}
}

// TestActiveBandsSeesQuadratureOnlyEnergy pins that a band whose energy sits
// entirely out of phase is not mistaken for silence and skipped.
func TestActiveBandsSeesQuadratureOnlyEnergy(t *testing.T) {
	t.Parallel()

	response, err := BandedFromEvents(
		[]Event{{TimeSeconds: 0, Amplitude: 1, PhaseRadians: math.Pi / 2}},
		phaseTestRenderConfig(),
	)
	if err != nil {
		t.Fatalf("BandedFromEvents: %v", err)
	}

	active := response.ActiveBands(-60)
	if len(active) != 1 || !active[0] {
		t.Fatalf("ActiveBands = %v, want the single band active", active)
	}
}
