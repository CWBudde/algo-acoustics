package ir

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
)

// RenderBinaural renders sparse events into dense left/right IR buffers.
func RenderBinaural(events []Event, hrtfDataset hrtf.Dataset, cfg RenderConfig) (left, right *Buffer, err error) {
	if hrtfDataset == nil {
		return nil, nil, errors.New("hrtf dataset must not be nil")
	}

	err = validateRenderConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	if sampleRate := hrtfDataset.SampleRate(); sampleRate > 0 && sampleRate != cfg.SampleRate {
		return nil, nil, fmt.Errorf("hrtf sample rate %d does not match render sample rate %d", sampleRate, cfg.SampleRate)
	}

	left = NewBuffer(cfg.SampleRate, cfg.DurationSeconds)
	right = NewBuffer(cfg.SampleRate, cfg.DurationSeconds)
	bandCount := cfg.BandSpec.BandCount()

	for index, event := range events {
		if event.TimeSeconds < 0 {
			return nil, nil, fmt.Errorf("event %d has negative time %g", index, event.TimeSeconds)
		}

		headDir := eventDirectionForHRTF(event.Direction)

		leftHRIR, rightHRIR, delaySeconds, lookupErr := hrtfDataset.Lookup(headDir)
		if lookupErr != nil {
			return nil, nil, fmt.Errorf("event %d hrtf lookup: %w", index, lookupErr)
		}

		eventGain, gainErr := monoEventGain(event, bandCount)
		if gainErr != nil {
			return nil, nil, fmt.Errorf("event %d: %w", index, gainErr)
		}

		startIndex := int(math.Round((event.TimeSeconds + delaySeconds) * float64(cfg.SampleRate)))
		left.Samples = convolveInto(left.Samples, startIndex, eventGain, leftHRIR)
		right.Samples = convolveInto(right.Samples, startIndex, eventGain, rightHRIR)
	}

	return left, right, nil
}

func eventDirectionForHRTF(dir geometry.Vec3) geometry.Vec3 {
	if dir == geometry.Vec3Zero {
		return geometry.Vec3{X: 1}
	}

	return dir.Normalize()
}

func convolveInto(samples []float64, startIndex int, amplitude float64, hrir []float64) []float64 {
	if amplitude == 0 {
		return samples
	}

	if len(hrir) == 0 {
		return addSample(samples, startIndex, amplitude)
	}

	kernelStart := 0
	if startIndex < 0 {
		kernelStart = -startIndex
		startIndex = 0

		if kernelStart >= len(hrir) {
			return samples
		}
	}

	needed := startIndex + len(hrir) - kernelStart
	if needed > len(samples) {
		extended := make([]float64, needed)
		copy(extended, samples)
		samples = extended
	}

	for i := kernelStart; i < len(hrir); i++ {
		samples[startIndex+i-kernelStart] += amplitude * hrir[i]
	}

	return samples
}

func addSample(samples []float64, index int, value float64) []float64 {
	if index < 0 {
		return samples
	}

	if index >= len(samples) {
		extended := make([]float64, index+1)
		copy(extended, samples)
		samples = extended
	}

	samples[index] += value

	return samples
}
