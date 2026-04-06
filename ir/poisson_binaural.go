package ir

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	algofft "github.com/cwbudde/algo-fft"
)

// BinauralPoissonConfig configures the binaural Poisson late-field synthesis.
type BinauralPoissonConfig struct {
	Bins        []EnergyBin
	BinDuration float64
	Volume      float64
	BandSpec    acoustics.BandSpec
	SampleRate  int
	HRTF        hrtf.Dataset
}

// RenderBinauralPoisson synthesises binaural late-field impulse responses from
// a Poisson noise process shaped by banded energy histogram envelopes.
//
// Algorithm:
//  1. Generate a single Poisson Dirac delta sequence.
//  2. FFT → per-band bandpass filtering → IFFT (same as mono).
//  3. For each time slot, weight bands by energy envelope (same as mono).
//  4. For each time slot, select a random direction and look up the HRIR.
//  5. Convolve the slot's weighted signal with left/right HRIRs.
//  6. Overlap-add with a 50% Hanning window into continuous left/right channels.
func RenderBinauralPoisson(cfg BinauralPoissonConfig, rng *rand.Rand) (left, right *Buffer, err error) {
	bandCount := cfg.BandSpec.BandCount()
	if bandCount == 0 {
		return nil, nil, errors.New("band spec has no bands")
	}

	if cfg.SampleRate <= 0 {
		return nil, nil, errors.New("sample rate must be positive")
	}

	if cfg.HRTF == nil {
		return nil, nil, errors.New("HRTF dataset must not be nil")
	}

	if len(cfg.Bins) == 0 {
		empty := NewBuffer(cfg.SampleRate, 0)
		return empty, empty, nil
	}

	duration := cfg.Bins[len(cfg.Bins)-1].TimeSeconds + cfg.BinDuration
	bufLen := int(math.Ceil(duration * float64(cfg.SampleRate)))
	fftSize := nextPow2(2 * bufLen)

	plan, poissonSpectrum, err := poissonFFT(cfg.Volume, cfg.SampleRate, duration, bufLen, fftSize, rng)
	if err != nil {
		return nil, nil, err
	}

	bandWeights := buildStrictBandWeights(cfg.BandSpec, fftSize, cfg.SampleRate)

	monoWeighted, err := buildBinauralMonoWeighted(plan, poissonSpectrum, bandWeights, cfg, bandCount, bufLen, fftSize)
	if err != nil {
		return nil, nil, err
	}

	// Step 4–6: per-slot HRIR convolution with overlap-add.
	// Allocate output buffers with extra room for HRIR tail.
	outLen := bufLen + cfg.SampleRate/10 // extra 100 ms for HRIR tails
	leftSamples := make([]float64, outLen)
	rightSamples := make([]float64, outLen)

	slotSamples := max(int(math.Round(cfg.BinDuration*float64(cfg.SampleRate))), 1)

	// Build Hanning window for overlap-add (50% overlap).
	halfSlot := slotSamples / 2
	windowLen := slotSamples + halfSlot // extended window for 50% overlap
	window := buildHanningWindow(windowLen)

	for _, bin := range cfg.Bins {
		slotStart := int(math.Round(bin.TimeSeconds * float64(cfg.SampleRate)))
		if slotStart >= bufLen {
			continue
		}

		err = convolveBinSlot(cfg, bin, rng, slotStart, bufLen, halfSlot, windowLen,
			window, monoWeighted, leftSamples, rightSamples, outLen)
		if err != nil {
			return nil, nil, err
		}
	}

	// Trim output to actual buffer length.
	if outLen > bufLen {
		leftSamples = leftSamples[:bufLen]
		rightSamples = rightSamples[:bufLen]
	}

	left = &Buffer{SampleRate: cfg.SampleRate, Samples: leftSamples}
	right = &Buffer{SampleRate: cfg.SampleRate, Samples: rightSamples}

	return left, right, nil
}

// buildBinauralMonoWeighted filters the Poisson spectrum into per-band
// sequences, applies the energy envelope, and sums bands into a single mono
// weighted sequence ready for HRIR spatialization.
func buildBinauralMonoWeighted(
	plan *algofft.Plan[complex128], poissonSpectrum []complex128, bandWeights [][]float64,
	cfg BinauralPoissonConfig, bandCount, bufLen, fftSize int,
) ([]float64, error) {
	bandSequences := make([][]float64, bandCount)
	filtered := make([]complex128, fftSize)
	timeDomain := make([]complex128, fftSize)

	for b := range bandCount {
		for k := range fftSize {
			filtered[k] = poissonSpectrum[k] * complex(bandWeights[b][k], 0)
		}

		err := plan.Inverse(timeDomain, filtered)
		if err != nil {
			return nil, fmt.Errorf("IFFT band %d: %w", b, err)
		}

		seq := make([]float64, bufLen)
		for i := range bufLen {
			seq[i] = real(timeDomain[i])
		}

		applyEnergyEnvelope(seq, cfg.Bins, b, cfg.BinDuration, cfg.SampleRate)

		bandSequences[b] = seq
	}

	monoWeighted := make([]float64, bufLen)
	for b := range bandCount {
		for i := range bufLen {
			monoWeighted[i] += bandSequences[b][i]
		}
	}

	return monoWeighted, nil
}

// convolveBinSlot spatialises one time slot by convolving the windowed mono
// signal with the left/right HRIRs for a random direction and overlap-adding
// the result into the output buffers.
func convolveBinSlot(
	cfg BinauralPoissonConfig, bin EnergyBin, rng *rand.Rand,
	slotStart, bufLen, halfSlot, windowLen int,
	window, monoWeighted, leftSamples, rightSamples []float64, outLen int,
) error {
	dir := randomDirection(rng)

	leftHRIR, rightHRIR, delaySeconds, lookupErr := cfg.HRTF.Lookup(dir)
	if lookupErr != nil {
		return fmt.Errorf("HRTF lookup at t=%.4f: %w", bin.TimeSeconds, lookupErr)
	}

	delaySamples := int(math.Round(delaySeconds * float64(cfg.SampleRate)))

	for i := range windowLen {
		srcIdx := slotStart - halfSlot + i
		if srcIdx < 0 || srcIdx >= bufLen {
			continue
		}

		sample := monoWeighted[srcIdx] * window[i]
		writeStart := srcIdx + delaySamples

		for h, lv := range leftHRIR {
			outIdx := writeStart + h
			if outIdx >= 0 && outIdx < outLen {
				leftSamples[outIdx] += sample * lv
			}
		}

		for h, rv := range rightHRIR {
			outIdx := writeStart + h
			if outIdx >= 0 && outIdx < outLen {
				rightSamples[outIdx] += sample * rv
			}
		}
	}

	return nil
}

// randomDirection generates a uniformly distributed random direction on the
// unit sphere using Marsaglia's method.
func randomDirection(rng *rand.Rand) geometry.Vec3 {
	for {
		u := 2*rng.Float64() - 1
		v := 2*rng.Float64() - 1
		s := u*u + v*v

		if s >= 1 {
			continue
		}

		factor := 2 * math.Sqrt(1-s)

		return geometry.Vec3{
			X: u * factor,
			Y: v * factor,
			Z: 1 - 2*s,
		}
	}
}

// buildHanningWindow creates a Hanning window of the given length.
func buildHanningWindow(length int) []float64 {
	w := make([]float64, length)
	for i := range length {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(length-1)))
	}

	return w
}
