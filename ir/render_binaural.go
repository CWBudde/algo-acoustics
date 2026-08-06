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

	for index, event := range events {
		if event.TimeSeconds < 0 {
			return nil, nil, fmt.Errorf("event %d has negative time %g", index, event.TimeSeconds)
		}

		if event.TimeSeconds >= cfg.DurationSeconds {
			continue
		}

		headDir := eventDirectionForHRTF(event.Direction)

		leftHRIR, rightHRIR, delaySeconds, lookupErr := hrtfDataset.Lookup(headDir)
		if lookupErr != nil {
			return nil, nil, fmt.Errorf("event %d hrtf lookup: %w", index, lookupErr)
		}

		excitation, renderErr := RenderMono([]Event{event}, cfg)
		if renderErr != nil {
			return nil, nil, fmt.Errorf("event %d: %w", index, renderErr)
		}

		convolveSignalInto(left.Samples, excitation.Samples, delaySeconds, cfg.SampleRate, leftHRIR)
		convolveSignalInto(right.Samples, excitation.Samples, delaySeconds, cfg.SampleRate, rightHRIR)
	}

	return left, right, nil
}

func eventDirectionForHRTF(dir geometry.Vec3) geometry.Vec3 {
	if dir == geometry.Vec3Zero {
		return geometry.Vec3{X: 1}
	}

	return dir.Normalize()
}

func convolveSignalInto(samples, signal []float64, delaySeconds float64, sampleRate int, hrir []float64) {
	if len(hrir) == 0 {
		hrir = []float64{1}
	}

	// Bound the delay before converting it to int. This both avoids undefinedly
	// large timestamp allocations and keeps conversion safe for extreme values.
	maxDelay := float64(len(samples)+len(signal)+len(hrir)) / float64(sampleRate)
	if math.IsNaN(delaySeconds) || delaySeconds >= maxDelay || delaySeconds <= -maxDelay {
		return
	}

	delaySamples := int(math.Round(delaySeconds * float64(sampleRate)))

	for signalIndex, amplitude := range signal {
		if amplitude == 0 {
			continue
		}

		for hrirIndex, coefficient := range hrir {
			outputIndex := signalIndex + delaySamples + hrirIndex
			if outputIndex >= 0 && outputIndex < len(samples) {
				samples[outputIndex] += amplitude * coefficient
			}
		}
	}
}
