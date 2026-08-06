package metrics

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-acoustics/ir"
)

const iaccMaxLagSeconds = 0.001 // ±1 ms interaural lag window

// IACC computes the Interaural Cross-Correlation Coefficient from binaural
// impulse responses. It returns the maximum absolute value of the normalized
// cross-correlation within a ±1 ms lag window:
//
//	IACC = max |ρ(τ)| for |τ| ≤ 1 ms
//
// where ρ(τ) = Σ left(t)·right(t+τ) / √(Σ left²(t) · Σ right²(t)).
//
// The result is in [0, 1]: 1 = fully correlated, 0 = uncorrelated.
func IACC(left, right *ir.Buffer) (float64, error) {
	err := validateIACCInputs(left, right)
	if err != nil {
		return 0, err
	}

	n := len(left.Samples)

	var leftEnergy, rightEnergy float64

	for i := range n {
		leftEnergy += left.Samples[i] * left.Samples[i]
		rightEnergy += right.Samples[i] * right.Samples[i]
	}

	if leftEnergy == 0 || rightEnergy == 0 {
		return 0, errors.New("buffers contain no energy")
	}

	normFactor := math.Sqrt(leftEnergy * rightEnergy)
	maxLag := int(math.Ceil(iaccMaxLagSeconds * float64(left.SampleRate)))

	var maxCorr float64

	for tau := -maxLag; tau <= maxLag; tau++ {
		var sum float64

		for i := range n {
			j := i + tau
			if j < 0 || j >= n {
				continue
			}

			sum += left.Samples[i] * right.Samples[j]
		}

		corr := math.Abs(sum / normFactor)
		if corr > maxCorr {
			maxCorr = corr
		}
	}

	return maxCorr, nil
}

func validateIACCInputs(left, right *ir.Buffer) error {
	if left == nil || right == nil {
		return errors.New("buffers must not be nil")
	}

	if left.SampleRate <= 0 || right.SampleRate <= 0 {
		return errors.New("buffer sample rates must be positive")
	}

	if left.SampleRate != right.SampleRate {
		return errors.New("buffer sample rates must match")
	}

	if len(left.Samples) != len(right.Samples) {
		return errors.New("buffer lengths must match")
	}

	if len(left.Samples) == 0 {
		return errors.New("buffers must not be empty")
	}

	return nil
}
