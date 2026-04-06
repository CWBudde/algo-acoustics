package raytrace

import (
	"math"
	"testing"
)

func TestDGHitProbabilitiesNormalization(t *testing.T) {
	// Two DGs with known energy: probabilities should sum to 1 per slot.
	dgs := make([]DirectivityGroup, 2)
	dgs[0].Histogram = NewEnergyHistogram(0.1, 0.05, 2)
	dgs[1].Histogram = NewEnergyHistogram(0.1, 0.05, 2)

	// Slot 0: DG0 gets 3.0 total, DG1 gets 1.0 total.
	dgs[0].Histogram.Add(0.01, []float64{2.0, 1.0})
	dgs[1].Histogram.Add(0.01, []float64{0.5, 0.5})

	probs := DGHitProbabilities(dgs)

	if len(probs) != 2 {
		t.Fatalf("expected 2 rows (one per DG), got %d", len(probs))
	}

	if len(probs[0]) != 2 {
		t.Fatalf("expected 2 columns (one per slot), got %d", len(probs[0]))
	}

	// Slot 0: P(DG0) = 3.0/4.0 = 0.75, P(DG1) = 1.0/4.0 = 0.25.
	if math.Abs(probs[0][0]-0.75) > 1e-10 {
		t.Errorf("P(DG0, slot0) = %f, want 0.75", probs[0][0])
	}

	if math.Abs(probs[1][0]-0.25) > 1e-10 {
		t.Errorf("P(DG1, slot0) = %f, want 0.25", probs[1][0])
	}

	// Probabilities should sum to 1 for each slot.
	for slot := range probs[0] {
		var sum float64

		for dg := range probs {
			sum += probs[dg][slot]
		}

		if math.Abs(sum-1.0) > 1e-10 {
			t.Errorf("slot %d: probability sum = %f, want 1.0", slot, sum)
		}
	}
}

func TestDGHitProbabilitiesEmptySlot(t *testing.T) {
	// Two DGs, slot 1 has no energy — should distribute uniformly.
	dgs := make([]DirectivityGroup, 3)

	for i := range dgs {
		dgs[i].Histogram = NewEnergyHistogram(0.1, 0.05, 1)
	}

	// Only add energy to slot 0.
	dgs[0].Histogram.Add(0.01, []float64{1.0})
	dgs[1].Histogram.Add(0.01, []float64{2.0})
	dgs[2].Histogram.Add(0.01, []float64{3.0})

	// Slot 1 has no energy in any DG.

	probs := DGHitProbabilities(dgs)

	// Slot 1: uniform = 1/3.
	uniform := 1.0 / 3.0

	for dg := range probs {
		if math.Abs(probs[dg][1]-uniform) > 1e-10 {
			t.Errorf("empty slot: P(DG%d, slot1) = %f, want %f", dg, probs[dg][1], uniform)
		}
	}
}

func TestDGHitProbabilitiesSingleDGDominance(t *testing.T) {
	// All energy in one DG — it should have probability 1.0.
	dgs := make([]DirectivityGroup, 4)

	for i := range dgs {
		dgs[i].Histogram = NewEnergyHistogram(0.05, 0.05, 2)
	}

	dgs[2].Histogram.Add(0.01, []float64{5.0, 3.0})

	probs := DGHitProbabilities(dgs)

	if math.Abs(probs[2][0]-1.0) > 1e-10 {
		t.Errorf("dominant DG probability = %f, want 1.0", probs[2][0])
	}

	for i := range dgs {
		if i == 2 {
			continue
		}

		if probs[i][0] != 0 {
			t.Errorf("non-dominant DG%d probability = %f, want 0.0", i, probs[i][0])
		}
	}
}

func TestDGHitProbabilitiesNilHistograms(t *testing.T) {
	dgs := make([]DirectivityGroup, 2)
	// Histograms are nil.

	probs := DGHitProbabilities(dgs)

	if probs != nil {
		t.Errorf("expected nil for DGs with nil histograms, got %v", probs)
	}
}

func TestDGHitProbabilitiesEmptyDGs(t *testing.T) {
	probs := DGHitProbabilities(nil)

	if probs != nil {
		t.Errorf("expected nil for nil DGs, got %v", probs)
	}
}

func TestDGHitProbabilitiesMultiBand(t *testing.T) {
	// Energy is summed across bands for each slot.
	dgs := make([]DirectivityGroup, 2)
	dgs[0].Histogram = NewEnergyHistogram(0.05, 0.05, 3)
	dgs[1].Histogram = NewEnergyHistogram(0.05, 0.05, 3)

	// DG0: bands sum to 6.0, DG1: bands sum to 4.0 → P(DG0) = 0.6, P(DG1) = 0.4.
	dgs[0].Histogram.Add(0.01, []float64{1.0, 2.0, 3.0})
	dgs[1].Histogram.Add(0.01, []float64{2.0, 1.0, 1.0})

	probs := DGHitProbabilities(dgs)

	if math.Abs(probs[0][0]-0.6) > 1e-10 {
		t.Errorf("P(DG0) = %f, want 0.6", probs[0][0])
	}

	if math.Abs(probs[1][0]-0.4) > 1e-10 {
		t.Errorf("P(DG1) = %f, want 0.4", probs[1][0])
	}
}
