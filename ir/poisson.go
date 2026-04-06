package ir

import (
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

const maxMu = 10000.0 // maximum event rate in s⁻¹ to prevent rattling artifacts

// PoissonSequence generates a Poisson-distributed Dirac delta sequence for
// late-field impulse response synthesis.
//
// The mean event rate follows the theoretical reflection density:
//
//	mu(t) = 4 * pi * c³ * t² / V
//
// No events are generated before the minimum time t0, where:
//
//	t0 = (2 * V * ln2 / (4 * pi * c³))^(1/3)
//
// Event times are generated exactly via the integrated-rate (inverse CDF)
// method. The cumulative rate function Lambda(t) = k*(t³ - t0³)/3, where
// k = 4*pi*c³/V, is inverted to find the next event time from a unit
// exponential variate. When mu(t) reaches the cap of 10,000 s⁻¹, the process
// switches to a constant-rate regime.
//
// Each event is a +1 or -1 Dirac delta, with at most one per sample. Sign is
// determined by the event's position within the sample: first half positive,
// second half negative.
func PoissonSequence(volume float64, sampleRate int, duration float64, rng *rand.Rand) []float64 {
	if volume <= 0 || sampleRate <= 0 || duration <= 0 {
		return nil
	}

	c := acoustics.SpeedOfSound
	k := 4 * math.Pi * c * c * c / volume

	// Minimum time: below this, the expected reflection density is less than
	// one event per time step, so the Poisson model is not meaningful.
	t0 := math.Cbrt(2 * volume * math.Ln2 / (k * volume))

	// Time at which mu(t) = k*t² hits the cap.
	tCap := math.Sqrt(maxMu / k)

	sampleCount := int(math.Ceil(duration * float64(sampleRate)))
	samples := make([]float64, sampleCount)
	sampleDuration := 1.0 / float64(sampleRate)

	// Cumulative rate from t0: Lambda(t) = k*(t³ - t0³)/3 for t <= tCap.
	// For t > tCap: Lambda(t) = Lambda(tCap) + maxMu*(t - tCap).
	lambdaCap := k * (tCap*tCap*tCap - t0*t0*t0) / 3

	lambda := 0.0 // accumulated integrated rate

	for {
		// Draw unit exponential increment.
		z := rng.Float64()
		if z == 0 {
			z = math.SmallestNonzeroFloat64
		}

		lambda += -math.Log(z)

		// Invert Lambda to find event time.
		var t float64
		if lambda <= lambdaCap {
			// Quadratic regime: Lambda = k*(t³ - t0³)/3
			// t = (3*Lambda/k + t0³)^(1/3)
			t = math.Cbrt(3*lambda/k + t0*t0*t0)
		} else {
			// Capped regime: t = tCap + (Lambda - lambdaCap) / maxMu
			t = tCap + (lambda-lambdaCap)/maxMu
		}

		if t >= duration {
			break
		}

		// Map to sample index; at most one event per sample.
		idx := int(math.Floor(t * float64(sampleRate)))
		if idx < 0 || idx >= sampleCount {
			continue
		}

		if samples[idx] != 0 {
			continue
		}

		// Sign: first half of sample interval → +1, second half → −1.
		sampleStart := float64(idx) * sampleDuration
		frac := (t - sampleStart) / sampleDuration

		if frac < 0.5 {
			samples[idx] = 1
		} else {
			samples[idx] = -1
		}
	}

	return samples
}
