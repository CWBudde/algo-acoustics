package ir

import "math"

// NormalizePeak scales the buffer so the absolute peak reaches 1.0.
// It returns the applied scale factor, or 0 if normalization was not possible.
func NormalizePeak(buf *Buffer) float64 {
	if buf == nil || len(buf.Samples) == 0 {
		return 0
	}

	peak := 0.0
	for _, sample := range buf.Samples {
		magnitude := math.Abs(sample)
		if magnitude > peak {
			peak = magnitude
		}
	}
	if peak == 0 {
		return 0
	}

	scale := 1 / peak
	for index := range buf.Samples {
		buf.Samples[index] *= scale
	}

	return scale
}

// NormalizeRMS scales the buffer to the requested RMS level.
// It returns the applied scale factor, or 0 if normalization was not possible.
func NormalizeRMS(buf *Buffer, targetRMS float64) float64 {
	if buf == nil || len(buf.Samples) == 0 || targetRMS <= 0 {
		return 0
	}

	energy := 0.0
	for _, sample := range buf.Samples {
		energy += sample * sample
	}

	rms := math.Sqrt(energy / float64(len(buf.Samples)))
	if rms == 0 {
		return 0
	}

	scale := targetRMS / rms
	for index := range buf.Samples {
		buf.Samples[index] *= scale
	}

	return scale
}
