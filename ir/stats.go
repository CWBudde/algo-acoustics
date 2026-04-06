package ir

import "math"

// BufferStats holds basic amplitude statistics for a rendered buffer.
type BufferStats struct {
	PeakAmplitude  float64
	RMSAmplitude   float64
	FirstArrivalMs float64
}

// Stats computes peak amplitude, RMS amplitude, and the time of the first
// non-silent sample in the buffer.
func Stats(buf *Buffer) BufferStats {
	if buf == nil || len(buf.Samples) == 0 || buf.SampleRate <= 0 {
		return BufferStats{}
	}

	var (
		peak              float64
		energySum         float64
		firstArrivalIndex = -1
	)

	for index, sample := range buf.Samples {
		magnitude := math.Abs(sample)
		if magnitude > peak {
			peak = magnitude
		}

		energySum += sample * sample

		if firstArrivalIndex < 0 && magnitude > 1e-9 {
			firstArrivalIndex = index
		}
	}

	stats := BufferStats{
		PeakAmplitude: peak,
		RMSAmplitude:  math.Sqrt(energySum / float64(len(buf.Samples))),
	}

	if firstArrivalIndex >= 0 {
		stats.FirstArrivalMs = float64(firstArrivalIndex) * 1000 / float64(buf.SampleRate)
	}

	return stats
}
