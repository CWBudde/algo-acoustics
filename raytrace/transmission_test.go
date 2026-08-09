package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestFilterTransmissionAppliesEnergyCoefficients(t *testing.T) {
	t.Parallel()

	histogram := NewEnergyHistogram(0.1, 0.05, 2)
	histogram.Add(0.01, []float64{4, 8})

	filtered, err := FilterTransmission(histogram, []float64{0.25, 0})
	if err != nil {
		t.Fatalf("FilterTransmission() error = %v", err)
	}

	if got := filtered.Bins[0].BandEnergy; got[0] != 1 || got[1] != 0 {
		t.Fatalf("filtered energy = %v, want [1 0]", got)
	}

	filtered.Bins[0].BandEnergy[0] = 99
	if histogram.Bins[0].BandEnergy[0] != 4 {
		t.Fatal("FilterTransmission() result aliases input")
	}
}

func TestFilterTransmissionBroadcastsAndRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	histogram := NewEnergyHistogram(0.1, 0.05, 2)
	histogram.Add(0.01, []float64{4, 8})

	filtered, err := FilterTransmission(histogram, []float64{0.5})
	if err != nil {
		t.Fatalf("FilterTransmission() error = %v", err)
	}

	if got := filtered.Bins[0].BandEnergy; got[0] != 2 || got[1] != 4 {
		t.Fatalf("filtered energy = %v, want [2 4]", got)
	}

	for _, transmission := range [][]float64{nil, {-0.1}, {1.1}, {math.NaN()}, {0.1, 0.2, 0.3}} {
		_, filterErr := FilterTransmission(histogram, transmission)
		if filterErr == nil {
			t.Fatalf("FilterTransmission(%v) error = nil, want error", transmission)
		}
	}
}

func TestEnergyEmissionsPreservesDelayAndCopiesEnergy(t *testing.T) {
	t.Parallel()

	histogram := NewEnergyHistogram(0.15, 0.05, 2)
	histogram.Add(0.06, []float64{1, 2})

	position := geometry.Vec3{X: 3, Y: 2, Z: 1}

	emissions := EnergyEmissions(histogram, position)
	if len(emissions) != 1 {
		t.Fatalf("EnergyEmissions() count = %d, want 1", len(emissions))
	}

	if emissions[0].Position != position || emissions[0].TimeSeconds != 0.05 {
		t.Fatalf("emission metadata = %#v", emissions[0])
	}

	emissions[0].BandEnergy[0] = 99
	if histogram.Bins[1].BandEnergy[0] != 1 {
		t.Fatal("EnergyEmissions() result aliases histogram")
	}
}
