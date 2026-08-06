package hybrid

import (
	"sort"

	"github.com/cwbudde/algo-acoustics/ir"
)

// CrossoverMode selects how the crossover point is determined.
type CrossoverMode int

const (
	// TimeBased uses an explicit crossover time.
	TimeBased CrossoverMode = iota
	// OrderBased uses the early-event order as a cutoff heuristic.
	OrderBased
	// EnergyBased chooses a cutoff once the early-event energy has mostly decayed.
	EnergyBased
)

// HybridConfig controls early/late crossover behavior.
type HybridConfig struct {
	CrossoverTimeSeconds float64
	CrossoverOrder       int
	CrossoverMode        CrossoverMode
	SmoothenCrossover    bool
	CrossoverWindow      FadeWindowConfig
}

// Combine merges early and late sparse events into a single ordered event list.
func Combine(early, late []ir.Event, cfg HybridConfig) []ir.Event {
	cutoff := effectiveCrossoverTime(early, cfg)

	out := make([]ir.Event, 0, len(early)+len(late))
	out = append(out, cloneEvents(early)...)

	for _, event := range late {
		if event.TimeSeconds < cutoff {
			continue
		}

		out = append(out, event)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TimeSeconds == out[j].TimeSeconds {
			return out[i].Kind < out[j].Kind
		}

		return out[i].TimeSeconds < out[j].TimeSeconds
	})

	return out
}

// CombineBuffers crossfades early and late buffers around the crossover point.
func CombineBuffers(early, late *ir.Buffer, cfg HybridConfig) *ir.Buffer {
	if early == nil && late == nil {
		return nil
	}

	if early == nil {
		return cloneBuffer(late)
	}

	if late == nil {
		return cloneBuffer(early)
	}

	if early.SampleRate <= 0 || late.SampleRate <= 0 || early.SampleRate != late.SampleRate {
		return nil
	}

	start, end := crossoverSampleRange(early, late, cfg)
	earlyOut, lateIn := crossfadeBuffers(early, late, start, end, cfg)

	out := cloneBuffer(earlyOut)
	if len(lateIn.Samples) > len(out.Samples) {
		resized := ir.NewBuffer(early.SampleRate, float64(len(lateIn.Samples))/float64(early.SampleRate))
		copy(resized.Samples, out.Samples)
		out = resized
	}

	for i := range lateIn.Samples {
		if i >= len(out.Samples) {
			break
		}

		out.Samples[i] += lateIn.Samples[i]
	}

	return out
}

func crossoverSampleRange(early, late *ir.Buffer, cfg HybridConfig) (int, int) {
	cutoff := min(max(int(cfg.CrossoverTimeSeconds*float64(early.SampleRate)), 0), len(early.Samples))
	windowSamples := crossoverWindowSamples(early.SampleRate)
	start := max(cutoff-windowSamples/2, 0)
	end := min(min(start+windowSamples, len(early.Samples)), len(late.Samples))

	return min(start, end), end
}

func crossfadeBuffers(early, late *ir.Buffer, start, end int, cfg HybridConfig) (*ir.Buffer, *ir.Buffer) {
	earlyOut := cloneBuffer(early)
	lateIn := cloneBuffer(late)

	if cfg.SmoothenCrossover {
		earlyOut = ApplyFadeWithWindow(earlyOut, start, end, false, cfg.CrossoverWindow)
		lateIn = ApplyFadeWithWindow(lateIn, start, end, true, cfg.CrossoverWindow)
	} else {
		applyLinearCrossfade(earlyOut, lateIn, start, end)
	}

	clear(earlyOut.Samples[end:])
	clear(lateIn.Samples[:start])

	return earlyOut, lateIn
}

func applyLinearCrossfade(early, late *ir.Buffer, start, end int) {
	span := maxInt(1, end-start-1)
	for i := start; i < end; i++ {
		weight := float64(i-start) / float64(span)
		early.Samples[i] *= 1 - weight
		late.Samples[i] *= weight
	}
}

func effectiveCrossoverTime(early []ir.Event, cfg HybridConfig) float64 {
	switch cfg.CrossoverMode {
	case OrderBased:
		if cfg.CrossoverOrder <= 0 || len(early) == 0 {
			return cfg.CrossoverTimeSeconds
		}

		sorted := cloneEvents(early)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].TimeSeconds < sorted[j].TimeSeconds })

		index := max(cfg.CrossoverOrder-1, 0)

		if index >= len(sorted) {
			index = len(sorted) - 1
		}

		return sorted[index].TimeSeconds
	case EnergyBased:
		if len(early) == 0 {
			return cfg.CrossoverTimeSeconds
		}

		sorted := cloneEvents(early)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].TimeSeconds < sorted[j].TimeSeconds })

		var total float64
		for _, event := range sorted {
			total += event.Amplitude * event.Amplitude
		}

		if total <= 0 {
			return cfg.CrossoverTimeSeconds
		}

		threshold := 0.9 * total

		var running float64
		for _, event := range sorted {
			running += event.Amplitude * event.Amplitude
			if running >= threshold {
				return event.TimeSeconds
			}
		}

		return sorted[len(sorted)-1].TimeSeconds
	default:
		return cfg.CrossoverTimeSeconds
	}
}

func crossoverWindowSamples(sampleRate int) int {
	window := max(int(0.01*float64(sampleRate)), 2)

	return window
}

func cloneEvents(events []ir.Event) []ir.Event {
	if len(events) == 0 {
		return nil
	}

	out := make([]ir.Event, len(events))
	copy(out, events)

	return out
}

func cloneBuffer(buf *ir.Buffer) *ir.Buffer {
	if buf == nil {
		return nil
	}

	out := &ir.Buffer{SampleRate: buf.SampleRate, Samples: make([]float64, len(buf.Samples))}
	copy(out.Samples, buf.Samples)

	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}
