package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestCombineStripsLateEventsBeforeCrossover(t *testing.T) {
	t.Parallel()

	early := []ir.Event{{TimeSeconds: 0.01}, {TimeSeconds: 0.02}}
	late := []ir.Event{{TimeSeconds: 0.015}, {TimeSeconds: 0.03}}

	combined := Combine(early, late, HybridConfig{CrossoverTimeSeconds: 0.02})
	if got, want := len(combined), 3; got != want {
		t.Fatalf("len(combined) = %d, want %d", got, want)
	}
	if combined[0].TimeSeconds != 0.01 || combined[1].TimeSeconds != 0.02 || combined[2].TimeSeconds != 0.03 {
		t.Fatalf("combined times = %#v", combined)
	}
}

func TestCombineBuffersCrossfadesAndKeepsLength(t *testing.T) {
	t.Parallel()

	early := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 100)}
	early.Samples[45] = 1
	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 100)}
	for i := 50; i < 100; i++ {
		late.Samples[i] = 2
	}

	combined := CombineBuffers(early, late, HybridConfig{CrossoverTimeSeconds: 0.05, SmoothenCrossover: true})
	if combined == nil {
		t.Fatal("CombineBuffers() = nil")
	}
	if got, want := len(combined.Samples), 100; got != want {
		t.Fatalf("len(combined.Samples) = %d, want %d", got, want)
	}
	if math.Abs(combined.Samples[45]-1) > 1e-12 {
		t.Fatalf("early region lost energy: %v", combined.Samples[45])
	}
	if combined.Samples[90] <= 0 {
		t.Fatalf("late region should remain positive, got %v", combined.Samples[90])
	}
}
