package raytrace

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
)

// FilterTransmission applies per-band energy transmission coefficients to a
// histogram. A single coefficient is broadcast across all bands.
func FilterTransmission(histogram *EnergyHistogram, transmission []float64) (*EnergyHistogram, error) {
	if histogram == nil {
		return nil, errors.New("energy histogram is nil")
	}

	if histogram.BandCount <= 0 {
		return nil, errors.New("energy histogram has no bands")
	}

	if len(transmission) != 1 && len(transmission) != histogram.BandCount {
		return nil, fmt.Errorf("transmission band count = %d, want 1 or %d", len(transmission), histogram.BandCount)
	}

	for index, coefficient := range transmission {
		if math.IsNaN(coefficient) || math.IsInf(coefficient, 0) || coefficient < 0 || coefficient > 1 {
			return nil, fmt.Errorf("transmission[%d] = %v, want finite and within [0, 1]", index, coefficient)
		}
	}

	filtered := NewEnergyHistogram(histogram.Duration, histogram.BinDuration, histogram.BandCount)
	for binIndex, bin := range histogram.Bins {
		energy := make([]float64, histogram.BandCount)
		for bandIndex := range energy {
			coefficient := transmission[0]
			if len(transmission) > 1 {
				coefficient = transmission[bandIndex]
			}

			if bandIndex < len(bin.BandEnergy) {
				energy[bandIndex] = bin.BandEnergy[bandIndex] * coefficient
			}
		}

		if binIndex < len(filtered.Bins) {
			filtered.Bins[binIndex].TimeSeconds = bin.TimeSeconds
			filtered.Bins[binIndex].BandEnergy = energy
		}
	}

	return filtered, nil
}

// EnergyEmissions converts non-silent histogram bins into delayed point-source
// emissions at position. Returned energy slices do not alias the histogram.
func EnergyEmissions(histogram *EnergyHistogram, position geometry.Vec3) []ir.EnergyEmission {
	if histogram == nil {
		return nil
	}

	emissions := make([]ir.EnergyEmission, 0, len(histogram.Bins))
	for _, bin := range histogram.Bins {
		if maxEnergy(bin.BandEnergy) <= 0 {
			continue
		}

		emissions = append(emissions, ir.EnergyEmission{
			Position:    position,
			TimeSeconds: bin.TimeSeconds,
			BandEnergy:  append([]float64(nil), bin.BandEnergy...),
		})
	}

	return emissions
}
