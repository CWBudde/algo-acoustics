package metrics

import "math"

// ApparentSoundReductionIndex computes the apparent sound reduction index
// between a source room and a receiving room:
//
//	R' = Ls - Lr + 10 log10(S / Ar)
//
// Levels are expressed in dB and areas in square metres. Invalid inputs return
// NaN because the established API intentionally returns only a scalar value.
func ApparentSoundReductionIndex(
	sourceLevel, receiverLevel, partitionArea, receiverAbsorptionArea float64,
) float64 {
	if !finiteTransmissionMetric(sourceLevel) || !finiteTransmissionMetric(receiverLevel) ||
		!positiveFiniteTransmissionMetric(partitionArea) ||
		!positiveFiniteTransmissionMetric(receiverAbsorptionArea) {
		return math.NaN()
	}

	return sourceLevel - receiverLevel + 10*math.Log10(partitionArea/receiverAbsorptionArea)
}

// FlankingApparentSoundReductionIndex combines the energy transmission
// coefficients of direct and flanking paths:
//
//	R' = -10 log10(sum(tau_ij))
//
// Each coefficient must be finite and within [0, 1]. An empty slice or a set
// of zero-transmission paths represents perfect isolation and returns +Inf.
// The sum is deliberately not clamped: several valid parallel paths may
// produce a negative apparent reduction index.
func FlankingApparentSoundReductionIndex(transmissionCoefficients []float64) float64 {
	totalTransmission := 0.0

	for _, coefficient := range transmissionCoefficients {
		if !finiteTransmissionMetric(coefficient) || coefficient < 0 || coefficient > 1 {
			return math.NaN()
		}

		totalTransmission += coefficient
	}

	if totalTransmission == 0 {
		return math.Inf(1)
	}

	return -10 * math.Log10(totalTransmission)
}

func finiteTransmissionMetric(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func positiveFiniteTransmissionMetric(value float64) bool {
	return value > 0 && finiteTransmissionMetric(value)
}
