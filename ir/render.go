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
//
// When events carry per-band gains (BandGain) and the config specifies a
// BandSpec, each band is rendered through a bandpass filter and summed. This
// preserves frequency-dependent phase inversions from the pressure reflectance
// model. Events without BandGain are rendered as wideband impulses.
//
// When no events have BandGain or no BandSpec is configured, the legacy scalar
// path is used (each event produces a single-sample impulse).
func RenderMono(events []Event, cfg RenderConfig) (*Buffer, error) {
	err := validateRenderConfig(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.BandSpec.BandCount() > 0 && hasBandedEvents(events) {
		return renderMonoBanded(events, cfg)
	}

	return renderMonoScalar(events, cfg)
}

// renderMonoScalar is the original delta-based rendering for events without
// frequency-dependent band gains.
func renderMonoScalar(events []Event, cfg RenderConfig) (*Buffer, error) {
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
	bandGain, err := aggregateBandGain(event.BandGain, bandCount)
	if err != nil {
		return 0, err
	}

	return event.Amplitude * bandGain * math.Cos(event.PhaseRadians), nil
}

func aggregateBandGain(bandGain []float64, bandCount int) (float64, error) {
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

	if sum == 0 {
		return 0, nil
	}

	return sum / float64(len(bandGain)), nil
}
