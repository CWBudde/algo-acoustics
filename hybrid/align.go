package hybrid

import (
	"math"

	"github.com/cwbudde/algo-acoustics/ir"
)

// AlignLateTail scales the late buffer so its post-crossover energy matches
// the early events in an equally sized pre-crossover window.
func AlignLateTail(late *ir.Buffer, earlyEvents []ir.Event, cfg HybridConfig) *ir.Buffer {
	if late == nil {
		return nil
	}

	aligned := cloneBuffer(late)
	if aligned.SampleRate <= 0 || len(aligned.Samples) == 0 {
		return aligned
	}

	cutoffSample := max(int(cfg.CrossoverTimeSeconds*float64(aligned.SampleRate)), 0)

	if cutoffSample >= len(aligned.Samples) {
		cutoffSample = len(aligned.Samples) - 1
	}

	window := crossoverWindowSamples(aligned.SampleRate)
	start := cutoffSample

	end := min(cutoffSample+window, len(aligned.Samples))

	cutoffSeconds := float64(cutoffSample) / float64(aligned.SampleRate)
	windowSeconds := float64(end-start) / float64(aligned.SampleRate)
	earlyEnergy := eventEnergyRMSInWindow(
		earlyEvents,
		max(cutoffSeconds-windowSeconds, 0),
		cutoffSeconds,
	)

	lateEnergy := bufferEnergyRMS(aligned, start, end)
	if earlyEnergy <= 0 || lateEnergy <= 0 {
		return aligned
	}

	scale := earlyEnergy / lateEnergy
	for i := range aligned.Samples {
		aligned.Samples[i] *= scale
	}

	return aligned
}

func eventEnergyRMS(events []ir.Event, cutoffSeconds float64) float64 {
	return eventEnergyRMSInWindow(events, 0, cutoffSeconds)
}

func eventEnergyRMSInWindow(events []ir.Event, startSeconds, endSeconds float64) float64 {
	var sum float64
	var count int

	for _, event := range events {
		if event.TimeSeconds < startSeconds || event.TimeSeconds > endSeconds {
			continue
		}

		bandEnergyGain := 1.0
		if len(event.BandGain) > 0 {
			bandEnergyGain = 0
			for _, gain := range event.BandGain {
				bandEnergyGain += gain * gain
			}

			bandEnergyGain /= float64(len(event.BandGain))
		}

		sum += event.Amplitude * event.Amplitude * bandEnergyGain
		count++
	}

	if count == 0 {
		return 0
	}

	return math.Sqrt(sum / float64(count))
}

func bufferEnergyRMS(buf *ir.Buffer, start, end int) float64 {
	if buf == nil || start < 0 || end <= start || start >= len(buf.Samples) {
		return 0
	}

	if end > len(buf.Samples) {
		end = len(buf.Samples)
	}
	var sum float64
	var count int

	for i := start; i < end; i++ {
		sample := buf.Samples[i]
		sum += sample * sample
		count++
	}

	if count == 0 {
		return 0
	}

	return math.Sqrt(sum / float64(count))
}
