package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
)

func phaseChainRenderConfig() ir.RenderConfig {
	return ir.RenderConfig{
		SampleRate:      8000,
		DurationSeconds: 0.01,
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{500},
			LowerEdges:  []float64{350},
			UpperEdges:  []float64{700},
		},
	}
}

func phaseChainFactor(t *testing.T, phase float64) *ir.BandedResponse {
	t.Helper()

	response, err := ir.BandedFromEvents(
		[]ir.Event{{TimeSeconds: 0, Amplitude: 1, PhaseRadians: phase}},
		phaseChainRenderConfig(),
	)
	if err != nil {
		t.Fatalf("BandedFromEvents: %v", err)
	}

	return response
}

// TestPathChainComposesPhasesAdditively is the pin for composing diffracted
// hops.
//
// Two quarter-wave factors must compose to phase pi, a full inversion. Under
// the old real-only representation each factor was projected to cos(pi/2) = 0
// before convolution, so their product vanished — a diffracted path through two
// mesh room groups simply disappeared.
func TestPathChainComposesPhasesAdditively(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		first      float64
		second     float64
		wantReal   float64
		wantQuadra float64
	}{
		{name: "quarter and quarter invert", first: math.Pi / 2, second: math.Pi / 2, wantReal: -1, wantQuadra: 0},
		{name: "quarter and zero stay quadrature", first: math.Pi / 2, second: 0, wantReal: 0, wantQuadra: 1},
		{name: "quarter and three quarters return", first: math.Pi / 2, second: -math.Pi / 2, wantReal: 1, wantQuadra: 0},
		{name: "phase free is unchanged", first: 0, second: 0, wantReal: 1, wantQuadra: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			chain := PathChain{
				Factors:     []*ir.BandedResponse{phaseChainFactor(t, test.first), phaseChainFactor(t, test.second)},
				PortalGains: []ScalarFilter{{1}},
			}

			resolved, err := chain.Resolve(0)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}

			if diff := math.Abs(resolved.Bands[0][0] - test.wantReal); diff > 1e-12 {
				t.Fatalf("in-phase component = %v, want %v", resolved.Bands[0][0], test.wantReal)
			}

			quadrature := 0.0
			if resolved.HasQuadrature() {
				quadrature = resolved.Quadrature[0][0]
			}

			if diff := math.Abs(quadrature - test.wantQuadra); diff > 1e-12 {
				t.Fatalf("quadrature component = %v, want %v", quadrature, test.wantQuadra)
			}
		})
	}
}

// TestPathChainKeepsPhaseFreeFactorsReal pins that a chain with no phase never
// pays for the three extra convolutions the complex form needs.
func TestPathChainKeepsPhaseFreeFactorsReal(t *testing.T) {
	t.Parallel()

	chain := PathChain{
		Factors:     []*ir.BandedResponse{phaseChainFactor(t, 0), phaseChainFactor(t, math.Pi)},
		PortalGains: []ScalarFilter{{1}},
	}

	resolved, err := chain.Resolve(0)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolved.HasQuadrature() {
		t.Fatal("a phase-free chain allocated a quadrature component")
	}

	if diff := math.Abs(resolved.Bands[0][0] + 1); diff > 1e-12 {
		t.Fatalf("in-phase component = %v, want -1", resolved.Bands[0][0])
	}
}
