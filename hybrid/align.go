package hybrid

import "github.com/cwbudde/algo-acoustics/ir"

// AlignLateTail scales the late buffer so its energy near the crossover matches the early tail.
func AlignLateTail(late *ir.Buffer, earlyEvents []ir.Event, cfg HybridConfig) *ir.Buffer {
	if late == nil {
		return nil
	}
	aligned := cloneBuffer(late)
	if aligned.SampleRate <= 0 || len(aligned.Samples) == 0 {
		return aligned
	}

	cutoffSample := int(cfg.CrossoverTimeSeconds * float64(aligned.SampleRate))
	if cutoffSample < 0 {
		cutoffSample = 0
	}
	if cutoffSample >= len(aligned.Samples) {
		cutoffSample = len(aligned.Samples) - 1
	}

	window := crossoverWindowSamples(aligned.SampleRate)
	start := cutoffSample
	end := cutoffSample + window
	if end > len(aligned.Samples) {
		end = len(aligned.Samples)
	}

	earlyEnergy := eventEnergyRMS(earlyEvents, cfg.CrossoverTimeSeconds)
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
	var sum float64
	var count int
	for _, event := range events {
		if event.TimeSeconds > cutoffSeconds {
			continue
		}
		sum += event.Amplitude * event.Amplitude
		count++
	}
	if count == 0 {
		return 0
	}
	return sqrt(sum / float64(count))
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
	return sqrt(sum / float64(count))
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	z := v
	for i := 0; i < 8; i++ {
		z = 0.5 * (z + v/z)
	}
	return z
}
