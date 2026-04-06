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

	// DG-weighted HRIR selection (optional). When both are set, the HRIR
	// direction for each time slot is selected from the directivity group
	// with the highest hit probability instead of using a random direction.
	DGDirections    []geometry.Vec3 // representative direction per DG
	DGProbabilities [][]float64     // probs[dg][slot]
	DGBlendCount    int             // 0 or 1 = max-probability; >1 = blend top-N DGs
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
		return NewBuffer(cfg.SampleRate, 0), NewBuffer(cfg.SampleRate, 0), nil
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

	for slotIdx, bin := range cfg.Bins {
		slotStart := int(math.Round(bin.TimeSeconds * float64(cfg.SampleRate)))
		if slotStart >= bufLen {
			continue
		}

		dir := slotDirection(cfg, slotIdx, rng)

		err = convolveBinSlotDir(cfg, dir, slotStart, bufLen, halfSlot, windowLen,
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

// slotDirection returns the HRIR direction for a given time slot. When DG data
// is configured, the direction is derived from the directivity group
// probabilities; otherwise a random direction is generated.
func slotDirection(cfg BinauralPoissonConfig, slotIdx int, rng *rand.Rand) geometry.Vec3 {
	if len(cfg.DGDirections) > 0 && len(cfg.DGProbabilities) > 0 {
		return dgDirectionForSlot(cfg.DGDirections, cfg.DGProbabilities, slotIdx, cfg.DGBlendCount)
	}

	return randomDirection(rng)
}

// convolveBinSlotDir spatialises one time slot by convolving the windowed mono
// signal with the left/right HRIRs for the given direction and overlap-adding
// the result into the output buffers.
func convolveBinSlotDir(
	cfg BinauralPoissonConfig, dir geometry.Vec3,
	slotStart, bufLen, halfSlot, windowLen int,
	window, monoWeighted, leftSamples, rightSamples []float64, outLen int,
) error {
	leftHRIR, rightHRIR, delaySeconds, lookupErr := cfg.HRTF.Lookup(dir)
	if lookupErr != nil {
		return fmt.Errorf("HRTF lookup: %w", lookupErr)
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

// dgDirectionForSlot returns the HRIR direction for the given time slot based
// on directivity group hit probabilities. When blendCount <= 1, it returns the
// direction of the DG with the highest probability. When blendCount > 1, it
// returns the probability-weighted average of the top-N DG directions.
func dgDirectionForSlot(dirs []geometry.Vec3, probs [][]float64, slotIdx, blendCount int) geometry.Vec3 {
	if len(dirs) == 0 || len(probs) == 0 {
		return geometry.Vec3{X: 1}
	}

	if blendCount <= 1 {
		bestIdx := 0
		bestProb := -1.0

		for d := range dirs {
			if d >= len(probs) || slotIdx >= len(probs[d]) {
				continue
			}

			if probs[d][slotIdx] > bestProb {
				bestProb = probs[d][slotIdx]
				bestIdx = d
			}
		}

		return dirs[bestIdx]
	}

	// Blend top-N: weighted average of directions by probability.
	var blended geometry.Vec3

	for d := range dirs {
		if d >= len(probs) || slotIdx >= len(probs[d]) {
			continue
		}

		w := probs[d][slotIdx]
		blended.X += dirs[d].X * w
		blended.Y += dirs[d].Y * w
		blended.Z += dirs[d].Z * w
	}

	norm := blended.Norm()
	if norm == 0 {
		return geometry.Vec3{X: 1}
	}

	return blended.Scale(1 / norm)
}

// buildHanningWindow creates a Hanning window of the given length.
func buildHanningWindow(length int) []float64 {
	w := make([]float64, length)
	for i := range length {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(length-1)))
	}

	return w
}
