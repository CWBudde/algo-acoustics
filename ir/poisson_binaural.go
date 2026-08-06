package ir

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"

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
	err = validateBinauralPoissonConfig(cfg, rng)
	if err != nil {
		return nil, nil, err
	}

	if len(cfg.Bins) == 0 {
		return NewBuffer(cfg.SampleRate, 0), NewBuffer(cfg.SampleRate, 0), nil
	}

	bandCount := cfg.BandSpec.BandCount()
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

	// A periodic Hann window with length twice the nominal slot uses the slot
	// duration as its 50% hop. Per-sample normalization below also preserves
	// unity gain at boundaries and for irregularly spaced histogram bins.
	windowLen := 2 * slotSamples
	frameOffset := -slotSamples / 2
	window := buildHanningWindow(windowLen)
	windowCoverage := buildWindowCoverage(cfg.Bins, cfg.SampleRate, frameOffset, window, bufLen)

	for slotIdx, bin := range cfg.Bins {
		slotStart := int(math.Round(bin.TimeSeconds * float64(cfg.SampleRate)))
		if slotStart >= bufLen {
			continue
		}

		dir := slotDirection(cfg, slotIdx, rng)

		err = convolveBinSlotDir(cfg, dir, slotStart, bufLen, frameOffset,
			window, windowCoverage, monoWeighted, leftSamples, rightSamples, outLen)
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

func validateBinauralPoissonConfig(cfg BinauralPoissonConfig, rng *rand.Rand) error {
	err := validatePoissonInputs(cfg.Bins, cfg.BinDuration, cfg.Volume, cfg.BandSpec, cfg.SampleRate, rng)
	if err != nil {
		return err
	}

	if cfg.HRTF == nil {
		return errors.New("HRTF dataset must not be nil")
	}

	hrtfSampleRate := cfg.HRTF.SampleRate()
	if hrtfSampleRate != cfg.SampleRate {
		return fmt.Errorf("HRTF sample rate %d does not match render sample rate %d", hrtfSampleRate, cfg.SampleRate)
	}

	return nil
}

func buildWindowCoverage(
	bins []EnergyBin,
	sampleRate, frameOffset int,
	window []float64,
	bufLen int,
) []float64 {
	coverage := make([]float64, bufLen)

	for _, bin := range bins {
		slotStart := int(math.Round(bin.TimeSeconds * float64(sampleRate)))
		for index, weight := range window {
			sourceIndex := slotStart + frameOffset + index
			if sourceIndex >= 0 && sourceIndex < bufLen {
				coverage[sourceIndex] += weight
			}
		}
	}

	return coverage
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
	slotStart, bufLen, frameOffset int,
	window, windowCoverage, monoWeighted, leftSamples, rightSamples []float64, outLen int,
) error {
	leftHRIR, rightHRIR, delaySeconds, lookupErr := cfg.HRTF.Lookup(dir)
	if lookupErr != nil {
		return fmt.Errorf("HRTF lookup: %w", lookupErr)
	}

	delaySamples := int(math.Round(delaySeconds * float64(cfg.SampleRate)))

	for i := range window {
		srcIdx := slotStart + frameOffset + i
		if srcIdx < 0 || srcIdx >= bufLen {
			continue
		}

		coverage := windowCoverage[srcIdx]
		if coverage <= 0 {
			continue
		}

		sample := monoWeighted[srcIdx] * window[i] / coverage
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
		return maxProbabilityDirection(dirs, probs, slotIdx)
	}

	return blendTopDirections(dirs, probs, slotIdx, blendCount)
}

type dgCandidate struct {
	index       int
	probability float64
}

func maxProbabilityDirection(dirs []geometry.Vec3, probs [][]float64, slotIdx int) geometry.Vec3 {
	bestIndex := 0
	bestProbability := -1.0

	for directionIndex := range dirs {
		if directionIndex >= len(probs) || slotIdx < 0 || slotIdx >= len(probs[directionIndex]) {
			continue
		}

		if probs[directionIndex][slotIdx] > bestProbability {
			bestProbability = probs[directionIndex][slotIdx]
			bestIndex = directionIndex
		}
	}

	return dirs[bestIndex]
}

func blendTopDirections(dirs []geometry.Vec3, probs [][]float64, slotIdx, blendCount int) geometry.Vec3 {
	candidates := make([]dgCandidate, 0, len(dirs))
	for directionIndex := range dirs {
		if directionIndex >= len(probs) || slotIdx < 0 || slotIdx >= len(probs[directionIndex]) {
			continue
		}

		probability := probs[directionIndex][slotIdx]
		if probability <= 0 || math.IsNaN(probability) || math.IsInf(probability, 0) {
			continue
		}

		candidates = append(candidates, dgCandidate{index: directionIndex, probability: probability})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].probability > candidates[j].probability
	})

	if blendCount > len(candidates) {
		blendCount = len(candidates)
	}

	var blended geometry.Vec3

	for _, candidate := range candidates[:blendCount] {
		direction := dirs[candidate.index]
		blended.X += direction.X * candidate.probability
		blended.Y += direction.Y * candidate.probability
		blended.Z += direction.Z * candidate.probability
	}

	norm := blended.Norm()
	if norm == 0 {
		return geometry.Vec3{X: 1}
	}

	return blended.Scale(1 / norm)
}

// buildHanningWindow creates a periodic Hann window of the given length. With
// a half-window hop, adjacent windows sum to one.
func buildHanningWindow(length int) []float64 {
	w := make([]float64, length)
	for i := range length {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(length)))
	}

	return w
}
