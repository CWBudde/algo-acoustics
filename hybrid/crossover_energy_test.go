package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

// TestCombineBuffersLinearCrossoverPreservesConstantSignal verifies that when
// both early and late buffers hold a constant unit signal, the output after a
// linear crossover is also exactly 1.0 at every sample. This tests that the
// blend weights sum to 1.0 at every point in the transition window.
func TestCombineBuffersLinearCrossoverPreservesConstantSignal(t *testing.T) {
	t.Parallel()

	const n = 1000
	early := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, n)}

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, n)}
	for i := range n {
		early.Samples[i] = 1.0
		late.Samples[i] = 1.0
	}

	combined := CombineBuffers(early, late, HybridConfig{
		CrossoverTimeSeconds: 0.5,
		SmoothenCrossover:    false,
	})
	if combined == nil {
		t.Fatal("CombineBuffers() = nil")
	}

	if got, want := len(combined.Samples), n; got != want {
		t.Fatalf("len(Samples) = %d, want %d", got, want)
	}

	for i, s := range combined.Samples {
		if math.Abs(s-1.0) > 1e-12 {
			t.Fatalf("Samples[%d] = %v, want 1.0 (linear blend must preserve constant signal)", i, s)
		}
	}
}

// TestCombineOrderBasedCrossoverUsesNthEventTime verifies that OrderBased mode
// resolves the crossover to the Nth early event's time, and that late events
// before that time are dropped.
func TestCombineOrderBasedCrossoverUsesNthEventTime(t *testing.T) {
	t.Parallel()

	early := []ir.Event{
		{TimeSeconds: 0.01},
		{TimeSeconds: 0.02},
		{TimeSeconds: 0.03},
		{TimeSeconds: 0.04},
	}
	// Late events: 0.005 and 0.025 are before the cutoff (t=0.03); 0.045 is after.
	late := []ir.Event{
		{TimeSeconds: 0.005},
		{TimeSeconds: 0.025},
		{TimeSeconds: 0.045},
	}

	combined := Combine(early, late, HybridConfig{
		CrossoverMode:  OrderBased,
		CrossoverOrder: 3, // cutoff = sorted[2].TimeSeconds = 0.03
	})

	// 4 early events + 1 late event that survives the cutoff.
	if got, want := len(combined), 5; got != want {
		t.Fatalf("len(combined) = %d, want %d (4 early + 1 late past 3rd-order cutoff)", got, want)
	}
}

// TestCombineEnergyBasedCrossoverFinds90PercentPoint verifies that EnergyBased
// mode sets the cutoff once 90 % of the early-event energy has accumulated,
// and that late events after the cutoff survive.
func TestCombineEnergyBasedCrossoverFinds90PercentPoint(t *testing.T) {
	t.Parallel()

	// 10 equal-amplitude events. Running energy crosses 90 % threshold at event 9
	// (t = 0.09s), so the cutoff is t = 0.09s.
	early := make([]ir.Event, 10)
	for i := range early {
		early[i] = ir.Event{TimeSeconds: float64(i+1) * 0.01, Amplitude: 1.0}
	}

	// A late event clearly after the 90 % cutoff.
	late := []ir.Event{{TimeSeconds: 0.095, Amplitude: 0.5}}

	combined := Combine(early, late, HybridConfig{CrossoverMode: EnergyBased})

	if got, want := len(combined), 11; got != want {
		t.Fatalf("len(combined) = %d, want %d (10 early + 1 late past 90%% cutoff)", got, want)
	}
}

// TestCombineBuffersNilEarlyReturnsCloneOfLate verifies the degenerate case
// where early is nil: the output must be a clone of late.
func TestCombineBuffersNilEarlyReturnsCloneOfLate(t *testing.T) {
	t.Parallel()

	late := &ir.Buffer{SampleRate: 1000, Samples: []float64{1, 2, 3}}

	out := CombineBuffers(nil, late, HybridConfig{})
	if out == nil {
		t.Fatal("CombineBuffers(nil, late) = nil, want clone of late")
	}

	if got, want := len(out.Samples), len(late.Samples); got != want {
		t.Fatalf("len(Samples) = %d, want %d", got, want)
	}

	for i, s := range out.Samples {
		if s != late.Samples[i] {
			t.Fatalf("Samples[%d] = %v, want %v (output must be a clone)", i, s, late.Samples[i])
		}
	}
}

// TestCombineBuffersMismatchedSampleRatesReturnsNil verifies that buffers with
// different sample rates are rejected to prevent silent resampling bugs.
func TestCombineBuffersMismatchedSampleRatesReturnsNil(t *testing.T) {
	t.Parallel()

	early := &ir.Buffer{SampleRate: 44100, Samples: make([]float64, 100)}
	late := &ir.Buffer{SampleRate: 48000, Samples: make([]float64, 100)}

	out := CombineBuffers(early, late, HybridConfig{CrossoverTimeSeconds: 0.05})
	if out != nil {
		t.Fatal("CombineBuffers with mismatched sample rates should return nil, got non-nil")
	}
}
