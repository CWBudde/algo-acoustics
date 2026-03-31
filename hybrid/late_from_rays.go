package hybrid

import (
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/raytrace"
)

// HistogramToEvents converts a late-energy histogram into sparse IR events.
func HistogramToEvents(h *raytrace.EnergyHistogram, rng *rand.Rand) []ir.Event {
	if h == nil || len(h.Bins) == 0 {
		return nil
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	events := make([]ir.Event, 0, len(h.Bins))
	for _, bin := range h.Bins {
		var totalEnergy float64
		for _, bandEnergy := range bin.BandEnergy {
			totalEnergy += bandEnergy
		}
		if totalEnergy <= 0 {
			continue
		}

		events = append(events, ir.Event{
			TimeSeconds:  bin.TimeSeconds,
			Amplitude:    totalEnergy,
			PhaseRadians: 2 * math.Pi * rng.Float64(),
			Kind:         ir.EventDiffuse,
		})
	}

	return events
}

// HistogramToBuffer converts a late-energy histogram into a mono IR buffer.
func HistogramToBuffer(h *raytrace.EnergyHistogram, sampleRate int) *ir.Buffer {
	if h == nil {
		return ir.NewBuffer(sampleRate, 0)
	}

	return h.ToLateMono(sampleRate)
}
