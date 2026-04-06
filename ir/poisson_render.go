package ir

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/acoustics"
	algofft "github.com/cwbudde/algo-fft"
)

// EnergyBin holds the per-band energy for one time slot of a histogram.
type EnergyBin struct {
	TimeSeconds float64
	BandEnergy  []float64
}

// PoissonConfig configures the Poisson late-field RIR synthesis.
type PoissonConfig struct {
	Bins        []EnergyBin
	BinDuration float64
	Volume      float64
	BandSpec    acoustics.BandSpec
	SampleRate  int
}

// RenderMonoPoisson synthesises a late-field mono impulse response from a
// Poisson noise process shaped by banded energy histogram envelopes.
//
// Algorithm:
//  1. Generate a Poisson Dirac delta sequence (volume-dependent density).
//  2. FFT the sequence; for each band multiply by bandpass weights → IFFT
//     to obtain one filtered noise sequence per band.
//  3. Weight each band's sequence sample-wise by the energy envelope:
//     s_i = v_i * sqrt(E_n(k) / sum(v_i^2 in slot k))
//  4. Sum all weighted bands to produce the monaural RIR.
func RenderMonoPoisson(cfg PoissonConfig, rng *rand.Rand) (*Buffer, error) {
	bandCount := cfg.BandSpec.BandCount()
	if bandCount == 0 {
		return nil, errors.New("band spec has no bands")
	}

	if cfg.SampleRate <= 0 {
		return nil, errors.New("sample rate must be positive")
	}

	if len(cfg.Bins) == 0 {
		return NewBuffer(cfg.SampleRate, 0), nil
	}

	duration := cfg.Bins[len(cfg.Bins)-1].TimeSeconds + cfg.BinDuration
	bufLen := int(math.Ceil(duration * float64(cfg.SampleRate)))
	fftSize := nextPow2(2 * bufLen)

	plan, poissonSpectrum, err := poissonFFT(cfg.Volume, cfg.SampleRate, duration, bufLen, fftSize, rng)
	if err != nil {
		return nil, err
	}

	bandWeights := buildStrictBandWeights(cfg.BandSpec, fftSize, cfg.SampleRate)

	// For each band: filter → IFFT → envelope weighting → accumulate.
	output := make([]float64, bufLen)
	filtered := make([]complex128, fftSize)
	timeDomain := make([]complex128, fftSize)

	for b := range bandCount {
		for k := range fftSize {
			filtered[k] = poissonSpectrum[k] * complex(bandWeights[b][k], 0)
		}

		err = plan.Inverse(timeDomain, filtered)
		if err != nil {
			return nil, fmt.Errorf("IFFT band %d: %w", b, err)
		}

		bandSeq := make([]float64, bufLen)
		for i := range bufLen {
			bandSeq[i] = real(timeDomain[i])
		}

		applyEnergyEnvelope(bandSeq, cfg.Bins, b, cfg.BinDuration, cfg.SampleRate)

		for i := range bufLen {
			output[i] += bandSeq[i]
		}
	}

	return &Buffer{SampleRate: cfg.SampleRate, Samples: output}, nil
}

// poissonFFT generates a Poisson sequence and returns its FFT plan and spectrum.
func poissonFFT(volume float64, sampleRate int, duration float64, bufLen, fftSize int, rng *rand.Rand) (*algofft.Plan[complex128], []complex128, error) {
	poissonRaw := PoissonSequence(volume, sampleRate, duration, rng)
	if len(poissonRaw) < bufLen {
		padded := make([]float64, bufLen)
		copy(padded, poissonRaw)
		poissonRaw = padded
	}

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, nil, fmt.Errorf("create FFT plan: %w", err)
	}

	poissonComplex := make([]complex128, fftSize)
	for i := range bufLen {
		poissonComplex[i] = complex(poissonRaw[i], 0)
	}

	spectrum := make([]complex128, fftSize)

	err = plan.Forward(spectrum, poissonComplex)
	if err != nil {
		return nil, nil, fmt.Errorf("FFT poisson sequence: %w", err)
	}

	return plan, spectrum, nil
}

// buildStrictBandWeights computes per-band frequency-domain weights without
// extending edge bands to DC or Nyquist. This ensures energy normalization is
// consistent with the band edges used for measurement. Adjacent bands still
// use half-cosine crossovers at their shared boundaries.
func buildStrictBandWeights(spec acoustics.BandSpec, fftSize, sampleRate int) [][]float64 {
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

		for b := range bandCount {
			lower := spec.LowerEdges[b]
			upper := spec.UpperEdges[b]

			if freq < lower || freq > upper {
				continue
			}

			// Crossover with previous band.
			if b > 0 {
				crossover := lower
				transLow := crossover * math.Pow(2, -0.25)
				transHigh := crossover * math.Pow(2, 0.25)

				if freq >= transLow && freq <= transHigh {
					x := logRatio(freq, transLow, transHigh)
					weights[b][k] = 0.5 * (1 - math.Cos(math.Pi*x))

					continue
				}
			}

			// Crossover with next band.
			if b < bandCount-1 {
				crossover := upper
				transLow := crossover * math.Pow(2, -0.25)
				transHigh := crossover * math.Pow(2, 0.25)

				if freq >= transLow && freq <= transHigh {
					x := logRatio(freq, transLow, transHigh)
					weights[b][k] = 0.5 * (1 + math.Cos(math.Pi*x))

					continue
				}
			}

			// Flat passband.
			weights[b][k] = 1
		}
	}

	return weights
}

// applyEnergyEnvelope weights the band sequence sample-wise by the histogram
// energy envelope for band b:
//
//	s_i = v_i * sqrt(E_n(k) / sum(v_i^2 in slot k))
//
// The normalization ensures the output energy in each time slot matches the
// histogram's energy for that band. No additional bandwidth scaling is needed
// because the input is already band-filtered.
func applyEnergyEnvelope(bandSeq []float64, bins []EnergyBin, band int, binDuration float64, sampleRate int) {
	for _, bin := range bins {
		if band >= len(bin.BandEnergy) {
			continue
		}

		energy := bin.BandEnergy[band]
		if energy <= 0 {
			// Zero energy → silence this slot.
			start := int(math.Round(bin.TimeSeconds * float64(sampleRate)))
			end := int(math.Round((bin.TimeSeconds + binDuration) * float64(sampleRate)))
			end = min(end, len(bandSeq))

			for i := max(start, 0); i < end; i++ {
				bandSeq[i] = 0
			}

			continue
		}

		start := int(math.Round(bin.TimeSeconds * float64(sampleRate)))
		end := int(math.Round((bin.TimeSeconds + binDuration) * float64(sampleRate)))
		end = min(end, len(bandSeq))

		// Compute sum of v_i^2 in this slot.
		var sumSq float64
		for i := max(start, 0); i < end; i++ {
			sumSq += bandSeq[i] * bandSeq[i]
		}

		if sumSq <= 0 {
			continue
		}

		scale := math.Sqrt(energy / sumSq)
		for i := max(start, 0); i < end; i++ {
			bandSeq[i] *= scale
		}
	}
}
