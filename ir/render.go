package ir

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

// RenderConfig configures sparse-to-dense IR rendering.
type RenderConfig struct {
	SampleRate      int
	DurationSeconds float64
	BandSpec        acoustics.BandSpec
}

// RenderMono renders sparse events into a dense mono impulse response buffer.
func RenderMono(events []Event, cfg RenderConfig) (*Buffer, error) {
	err := validateRenderConfig(cfg)
	if err != nil {
		return nil, err
	}

	buf := NewBuffer(cfg.SampleRate, cfg.DurationSeconds)
	bandCount := cfg.BandSpec.BandCount()

	for index, event := range events {
		if event.TimeSeconds < 0 {
			return nil, fmt.Errorf("event %d has negative time %g", index, event.TimeSeconds)
		}

		sampleIndex := int(math.Round(event.TimeSeconds * float64(cfg.SampleRate)))
		if sampleIndex < 0 || sampleIndex >= buf.Len() {
			continue
		}

		gain, err := monoEventGain(event, bandCount)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", index, err)
		}

		buf.Samples[sampleIndex] += gain
	}

	return buf, nil
}

func validateRenderConfig(cfg RenderConfig) error {
	if cfg.SampleRate <= 0 {
		return errors.New("sample rate must be positive")
	}

	if cfg.DurationSeconds <= 0 {
		return errors.New("duration must be positive")
	}

	bandCount := cfg.BandSpec.BandCount()
	if len(cfg.BandSpec.LowerEdges) != bandCount || len(cfg.BandSpec.UpperEdges) != bandCount {
		return errors.New("band spec lengths must match")
	}

	return nil
}

func monoEventGain(event Event, bandCount int) (float64, error) {
	bandSum, err := sumBandGain(event.BandGain, bandCount)
	if err != nil {
		return 0, err
	}

	return event.Amplitude * bandSum * math.Cos(event.PhaseRadians), nil
}

func sumBandGain(bandGain []float64, bandCount int) (float64, error) {
	if len(bandGain) == 0 {
		return 1, nil
	}

	if bandCount > 0 && len(bandGain) != bandCount {
		return 0, fmt.Errorf("band gain length %d does not match band count %d", len(bandGain), bandCount)
	}

	sum := 0.0
	for _, value := range bandGain {
		sum += value
	}

	return sum, nil
}
