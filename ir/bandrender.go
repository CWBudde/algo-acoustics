package ir

import (
	"errors"
	"fmt"
	"math"
)

// RenderBand renders sparse events into a dense mono buffer for one frequency band.
func RenderBand(events []Event, bandIndex int, cfg RenderConfig) (*Buffer, error) {
	if err := validateRenderConfig(cfg); err != nil {
		return nil, err
	}

	bandCount := cfg.BandSpec.BandCount()
	if bandCount <= 0 {
		return nil, errors.New("band spec must contain at least one band")
	}
	if bandIndex < 0 || bandIndex >= bandCount {
		return nil, fmt.Errorf("band index %d out of range [0,%d)", bandIndex, bandCount)
	}

	buf := NewBuffer(cfg.SampleRate, cfg.DurationSeconds)

	for index, event := range events {
		if event.TimeSeconds < 0 {
			return nil, fmt.Errorf("event %d has negative time %g", index, event.TimeSeconds)
		}

		sampleIndex := int(math.Round(event.TimeSeconds * float64(cfg.SampleRate)))
		if sampleIndex < 0 || sampleIndex >= buf.Len() {
			continue
		}

		gain, err := bandEventGain(event, bandIndex, bandCount)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", index, err)
		}

		buf.Samples[sampleIndex] += gain
	}

	return buf, nil
}

// SumBands sums aligned band buffers into a single dense buffer.
func SumBands(bands []*Buffer) *Buffer {
	var out *Buffer

	for _, band := range bands {
		if band == nil {
			continue
		}
		if out == nil {
			out = &Buffer{
				SampleRate: band.SampleRate,
				Samples:    append([]float64(nil), band.Samples...),
			}

			continue
		}
		if band.SampleRate != out.SampleRate || len(band.Samples) != len(out.Samples) {
			return nil
		}

		for index, sample := range band.Samples {
			out.Samples[index] += sample
		}
	}

	return out
}

// RenderHybridBand renders early and late events for one band and sums them.
func RenderHybridBand(earlyEvents, lateEvents []Event, bandIndex int, cfg RenderConfig) (*Buffer, error) {
	early, err := RenderBand(earlyEvents, bandIndex, cfg)
	if err != nil {
		return nil, err
	}
	late, err := RenderBand(lateEvents, bandIndex, cfg)
	if err != nil {
		return nil, err
	}

	return SumBands([]*Buffer{early, late}), nil
}

// SumBandsWeighted sums aligned band buffers after applying scalar weights.
func SumBandsWeighted(bands []*Buffer, weights []float64) *Buffer {
	var out *Buffer

	for index, band := range bands {
		if band == nil {
			continue
		}
		weight := 1.0
		if index < len(weights) {
			weight = weights[index]
		}
		if out == nil {
			out = &Buffer{SampleRate: band.SampleRate, Samples: make([]float64, len(band.Samples))}
		}
		if band.SampleRate != out.SampleRate || len(band.Samples) != len(out.Samples) {
			return nil
		}
		for sampleIndex, sample := range band.Samples {
			out.Samples[sampleIndex] += sample * weight
		}
	}

	return out
}

func bandEventGain(event Event, bandIndex, bandCount int) (float64, error) {
	bandGain, err := bandGainAt(event.BandGain, bandIndex, bandCount)
	if err != nil {
		return 0, err
	}

	return event.Amplitude * bandGain * math.Cos(event.PhaseRadians), nil
}

func bandGainAt(bandGain []float64, bandIndex, bandCount int) (float64, error) {
	if bandIndex < 0 || bandIndex >= bandCount {
		return 0, fmt.Errorf("band index %d out of range [0,%d)", bandIndex, bandCount)
	}
	if len(bandGain) == 0 {
		return 1, nil
	}
	if len(bandGain) != bandCount {
		return 0, fmt.Errorf("band gain length %d does not match band count %d", len(bandGain), bandCount)
	}

	return bandGain[bandIndex], nil
}
