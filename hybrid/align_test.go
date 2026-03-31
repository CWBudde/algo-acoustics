package hybrid

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestAlignLateTailMatchesEarlyEnergy(t *testing.T) {
	t.Parallel()

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 100)}
	for i := 20; i < 30; i++ {
		late.Samples[i] = 1
	}

	aligned := AlignLateTail(late, []ir.Event{{TimeSeconds: 0.02, Amplitude: 2}}, HybridConfig{CrossoverTimeSeconds: 0.02})
	if aligned == nil {
		t.Fatal("AlignLateTail() = nil")
	}

	if aligned.Samples[20] < 1.5 {
		t.Fatalf("aligned sample = %v, want scaled up", aligned.Samples[20])
	}
}
