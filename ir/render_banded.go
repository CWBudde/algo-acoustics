package ir

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
	algofft "github.com/cwbudde/algo-fft"
)

// renderMonoBanded renders events with per-band frequency filtering.
//
// Instead of averaging BandGain into a scalar, each band is rendered through
// a bandpass filter and summed. This preserves frequency-dependent phase
// inversions from the pressure reflectance model (Vorländer, "Auralization").
//
// Algorithm:
//  1. For each octave band, accumulate banded event impulses into a time buffer
//  2. FFT each band buffer, multiply by the bandpass weight, sum into combined spectrum
//  3. Add wideband events (no BandGain) as flat-spectrum impulses
//  4. IFFT the combined spectrum to produce the output buffer
func renderMonoBanded(events []Event, cfg RenderConfig) (*Buffer, error) {
	sampleRate := cfg.SampleRate
	bufLen := NewBuffer(sampleRate, cfg.DurationSeconds).Len()
	fftSize := nextPow2(2 * bufLen)
	bandCount := cfg.BandSpec.BandCount()

	err := validateBandedEvents(events, bandCount)
	if err != nil {
		return nil, err
	}

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("create FFT plan: %w", err)
	}

	bandWeights := buildBandpassWeights(cfg.BandSpec, fftSize, sampleRate)
	combined := make([]complex128, fftSize)
	scratch := make([]complex128, fftSize)

	err = accumulateBandedSpectrum(events, bandCount, sampleRate, bufLen, bandWeights, combined, scratch, plan)
	if err != nil {
		return nil, err
	}

	err = accumulateWidebandSpectrum(events, sampleRate, bufLen, combined, scratch, plan)
	if err != nil {
		return nil, err
	}

	timeDomain := make([]complex128, fftSize)

	err = plan.Inverse(timeDomain, combined)
	if err != nil {
		return nil, fmt.Errorf("IFFT: %w", err)
	}

	buf := NewBuffer(sampleRate, cfg.DurationSeconds)
	for i := range buf.Samples {
		buf.Samples[i] = real(timeDomain[i])
	}

	return buf, nil
}

func validateBandedEvents(events []Event, bandCount int) error {
	for index, event := range events {
		if event.TimeSeconds < 0 {
			return fmt.Errorf("event %d has negative time %g", index, event.TimeSeconds)
		}

		if len(event.BandGain) > 0 && len(event.BandGain) != bandCount {
			return fmt.Errorf("event %d: band gain length %d does not match band count %d",
				index, len(event.BandGain), bandCount)
		}
	}

	return nil
}

// accumulateBandedSpectrum renders events with BandGain through per-band
// bandpass filters and accumulates them into the combined spectrum.
func accumulateBandedSpectrum(events []Event, bandCount, sampleRate, bufLen int, bandWeights [][]float64, combined, scratch []complex128, plan *algofft.Plan[complex128]) error {
	bandTime := make([]complex128, len(combined))
	fftSize := len(combined)

	for b := range bandCount {
		clear(bandTime)

		for _, event := range events {
			if len(event.BandGain) == 0 {
				continue
			}

			sampleIndex := int(math.Round(event.TimeSeconds * float64(sampleRate)))
			if sampleIndex < 0 || sampleIndex >= bufLen {
				continue
			}

			gain := event.Amplitude * event.BandGain[b] * math.Cos(event.PhaseRadians)
			bandTime[sampleIndex] += complex(gain, 0)
		}

		err := plan.Forward(scratch[:fftSize], bandTime)
		if err != nil {
			return fmt.Errorf("FFT band %d: %w", b, err)
		}

		for k := range combined {
			combined[k] += scratch[k] * complex(bandWeights[b][k], 0)
		}
	}

	return nil
}

// accumulateWidebandSpectrum renders events without BandGain as flat-spectrum
// impulses and adds them to the combined spectrum.
func accumulateWidebandSpectrum(events []Event, sampleRate, bufLen int, combined, scratch []complex128, plan *algofft.Plan[complex128]) error {
	bandTime := make([]complex128, len(combined))
	hasWideband := false

	for _, event := range events {
		if len(event.BandGain) > 0 {
			continue
		}

		sampleIndex := int(math.Round(event.TimeSeconds * float64(sampleRate)))
		if sampleIndex < 0 || sampleIndex >= bufLen {
			continue
		}

		gain := event.Amplitude * math.Cos(event.PhaseRadians)
		bandTime[sampleIndex] += complex(gain, 0)
		hasWideband = true
	}

	if !hasWideband {
		return nil
	}

	err := plan.Forward(scratch, bandTime)
	if err != nil {
		return fmt.Errorf("FFT wideband: %w", err)
	}

	for k := range combined {
		combined[k] += scratch[k]
	}

	return nil
}

// buildBandpassWeights computes per-band frequency-domain weights for fftSize bins.
//
// Each band's passband is flat within its [LowerEdge, UpperEdge] range. At the
// boundaries between adjacent bands, a half-cosine crossover ensures smooth
// transitions and a partition of unity (weights sum to 1 at all frequencies).
// The lowest band extends to DC; the highest extends to Nyquist.
func buildBandpassWeights(spec acoustics.BandSpec, fftSize, sampleRate int) [][]float64 {
	bandCount := spec.BandCount()
	nyquist := float64(sampleRate) / 2

	weights := make([][]float64, bandCount)
	for b := range bandCount {
		weights[b] = make([]float64, fftSize)
	}

	for k := range fftSize {
		freq := float64(k) * float64(sampleRate) / float64(fftSize)
		if freq > nyquist {
			freq = float64(sampleRate) - freq
		}

		assignBandWeights(weights, k, freq, spec)
	}

	return weights
}

// assignBandWeights sets the weight for each band at frequency bin k.
func assignBandWeights(weights [][]float64, k int, freq float64, spec acoustics.BandSpec) {
	bandCount := spec.BandCount()

	if bandCount == 0 {
		return
	}

	// Below the lowest band's lower edge → all weight to band 0.
	if freq <= spec.LowerEdges[0] {
		weights[0][k] = 1
		return
	}

	// Above the highest band's upper edge → all weight to last band.
	if freq >= spec.UpperEdges[bandCount-1] {
		weights[bandCount-1][k] = 1
		return
	}

	for b := range bandCount {
		lower := spec.LowerEdges[b]
		upper := spec.UpperEdges[b]

		// Fully inside the band's passband (away from edges).
		if freq >= upper || freq <= lower {
			continue
		}

		// Check if we're in the crossover region with the previous band.
		if b > 0 && freq < lower {
			continue
		}

		// Check if we're near the lower crossover.
		if b > 0 {
			crossoverFreq := lower // = spec.UpperEdges[b-1]
			transitionLow := crossoverFreq * math.Pow(2, -0.25)
			transitionHigh := crossoverFreq * math.Pow(2, 0.25)

			if freq >= transitionLow && freq <= transitionHigh {
				x := logRatio(freq, transitionLow, transitionHigh)
				weights[b][k] = 0.5 * (1 - math.Cos(math.Pi*x))
				weights[b-1][k] = 0.5 * (1 + math.Cos(math.Pi*x))

				return
			}
		}

		// Check if we're near the upper crossover.
		if b < bandCount-1 {
			crossoverFreq := upper // = spec.LowerEdges[b+1]
			transitionLow := crossoverFreq * math.Pow(2, -0.25)
			transitionHigh := crossoverFreq * math.Pow(2, 0.25)

			if freq >= transitionLow && freq <= transitionHigh {
				x := logRatio(freq, transitionLow, transitionHigh)
				weights[b][k] = 0.5 * (1 + math.Cos(math.Pi*x))
				weights[b+1][k] = 0.5 * (1 - math.Cos(math.Pi*x))

				return
			}
		}

		// Flat passband interior.
		weights[b][k] = 1

		return
	}
}

// logRatio returns where freq falls in [low, high] on a log-frequency scale, in [0, 1].
func logRatio(freq, low, high float64) float64 {
	if freq <= low {
		return 0
	}

	if freq >= high {
		return 1
	}

	return math.Log2(freq/low) / math.Log2(high/low)
}

func nextPow2(n int) int {
	if n <= 1 {
		return 1
	}

	p := 1
	for p < n {
		p <<= 1
	}

	return p
}

func hasBandedEvents(events []Event) bool {
	for i := range events {
		if len(events[i].BandGain) > 0 {
			return true
		}
	}

	return false
}
