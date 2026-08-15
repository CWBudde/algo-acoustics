package algoacoustics

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	// maxComposedPathEvents caps the sparse event expansion of one path.
	maxComposedPathEvents = 4096
	// composedEventFloorDB drops composed events far below the strongest one.
	composedEventFloorDB = -80.0
)

// composePathEvents expands a path into sparse events by convolving the hops in
// the event domain: every combination of one event per hop, with times and
// distances added and amplitudes multiplied, and the portal filter applied at
// each handoff.
//
// This is the sparse counterpart of the banded convolution that RenderMono
// performs, and it produces the same result — convolving two impulse trains is
// exactly their cartesian product. It exists because callers that consume
// events directly, such as the event dump, need the sparse form.
//
// The expansion is inherently combinatorial, which is precisely why the dense
// render composes by convolution instead. It is bounded here by a relative
// level floor and a hard event cap, and the strongest contributions are kept.
func composePathEvents(
	sc *scene.Scene,
	path networkPath,
	factors []*GroupFactor,
	bandCount int,
) []ir.Event {
	if len(factors) == 0 {
		return nil
	}

	running := cloneComposedEvents(factors[0].Events, bandCount)

	for index, factor := range factors[1:] {
		gain := hybrid.PressureFilterFromTransmission(portalTransmission(sc, path.portals[index]))
		running = combineComposedEvents(running, factor.Events, gain, bandCount)
	}

	for index := range running {
		running[index].Kind = ir.EventTransmission
	}

	return running
}

// cloneComposedEvents normalises a hop's events so every one carries an
// explicit per-band gain, which keeps the combination step uniform.
func cloneComposedEvents(events []ir.Event, bandCount int) []ir.Event {
	out := make([]ir.Event, 0, len(events))

	for _, event := range events {
		copied := event
		copied.BandGain = expandBandGain(event.BandGain, bandCount)
		out = append(out, copied)
	}

	return out
}

// combineComposedEvents forms the cartesian product of two hops' events,
// applying the portal filter of the handoff between them.
func combineComposedEvents(running, next []ir.Event, gain hybrid.ScalarFilter, bandCount int) []ir.Event {
	combined := make([]ir.Event, 0, len(running)*len(next))

	for _, first := range running {
		for _, second := range next {
			event := ir.Event{
				TimeSeconds:    first.TimeSeconds + second.TimeSeconds,
				Amplitude:      first.Amplitude * second.Amplitude,
				Direction:      second.Direction,
				DistanceMeters: first.DistanceMeters + second.DistanceMeters,
				BandGain:       make([]float64, bandCount),
				PhaseRadians:   first.PhaseRadians + second.PhaseRadians,
				Kind:           ir.EventTransmission,
			}

			secondGain := expandBandGain(second.BandGain, bandCount)
			for band := range event.BandGain {
				value := first.BandGain[band] * secondGain[band]
				if band < len(gain) {
					value *= gain[band]
				}

				event.BandGain[band] = value
			}

			combined = append(combined, event)
		}
	}

	return pruneComposedEvents(combined)
}

// pruneComposedEvents keeps the strongest contributions, dropping those far
// below the peak and capping the total count.
func pruneComposedEvents(events []ir.Event) []ir.Event {
	if len(events) == 0 {
		return events
	}

	peak := 0.0
	for _, event := range events {
		peak = math.Max(peak, composedEventMagnitude(event))
	}

	if peak <= 0 {
		return nil
	}

	threshold := peak * math.Pow(10, composedEventFloorDB/20)

	kept := make([]ir.Event, 0, len(events))

	for _, event := range events {
		if composedEventMagnitude(event) > threshold {
			kept = append(kept, event)
		}
	}

	if len(kept) > maxComposedPathEvents {
		sort.SliceStable(kept, func(i, j int) bool {
			return composedEventMagnitude(kept[i]) > composedEventMagnitude(kept[j])
		})

		kept = kept[:maxComposedPathEvents]
	}

	return kept
}

func composedEventMagnitude(event ir.Event) float64 {
	peak := 0.0
	for _, gain := range event.BandGain {
		peak = math.Max(peak, math.Abs(gain))
	}

	if len(event.BandGain) == 0 {
		peak = 1
	}

	return math.Abs(event.Amplitude) * peak
}

func expandBandGain(gain []float64, bandCount int) []float64 {
	expanded := make([]float64, bandCount)
	for band := range expanded {
		if band < len(gain) {
			expanded[band] = gain[band]

			continue
		}

		expanded[band] = 1
	}

	return expanded
}
