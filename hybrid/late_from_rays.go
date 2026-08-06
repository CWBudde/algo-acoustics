package hybrid

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/raytrace"
)

// HistogramToEvents converts a late-energy histogram into sparse IR events.
func HistogramToEvents(h *raytrace.EnergyHistogram, rng *rand.Rand) []ir.Event {
	if h == nil || len(h.Bins) == 0 {
		return nil
	}

	if rng == nil {
		rng = rand.New(rand.NewSource(1)) // #nosec G404 -- deterministic simulation phases, not security randomness.
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
			Amplitude:    math.Sqrt(totalEnergy),
			PhaseRadians: 2 * math.Pi * rng.Float64(),
			Kind:         ir.EventDiffuse,
		})
	}

	return events
}

// HistogramToBuffer converts a late-energy histogram into a mono IR buffer
// using the legacy random-noise method.
func HistogramToBuffer(h *raytrace.EnergyHistogram, sampleRate int) *ir.Buffer {
	if h == nil {
		return ir.NewBuffer(sampleRate, 0)
	}

	return h.ToLateMono(sampleRate)
}

// HistogramToPoissonBuffer converts a late-energy histogram into a mono IR
// buffer using the Poisson noise process with per-band filtering.
func HistogramToPoissonBuffer(h *raytrace.EnergyHistogram, volume float64, spec acoustics.BandSpec, sampleRate int, rng *rand.Rand) (*ir.Buffer, error) {
	if h == nil || len(h.Bins) == 0 {
		return ir.NewBuffer(sampleRate, 0), nil
	}

	bins := make([]ir.EnergyBin, len(h.Bins))
	for i, hb := range h.Bins {
		bins[i] = ir.EnergyBin{
			TimeSeconds: hb.TimeSeconds,
			BandEnergy:  append([]float64(nil), hb.BandEnergy...),
		}
	}

	buf, err := ir.RenderMonoPoisson(ir.PoissonConfig{
		Bins:        bins,
		BinDuration: h.BinDuration,
		Volume:      volume,
		BandSpec:    spec,
		SampleRate:  sampleRate,
	}, rng)
	if err != nil {
		return nil, fmt.Errorf("render histogram Poisson buffer: %w", err)
	}

	return buf, nil
}
