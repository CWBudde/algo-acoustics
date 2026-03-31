package raytrace

import (
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/ir"
)

// HistogramBin stores the accumulated per-band energy for a time window.
type HistogramBin struct {
	TimeSeconds float64   `json:"timeSeconds"`
	BandEnergy  []float64 `json:"bandEnergy"`
}

// EnergyHistogram stores ray energy as banded time bins.
type EnergyHistogram struct {
	Bins        []HistogramBin
	BinDuration float64
	BandCount   int
}

// NewEnergyHistogram allocates a histogram for the requested duration.
func NewEnergyHistogram(duration, binDuration float64, bandCount int) *EnergyHistogram {
	if duration <= 0 || binDuration <= 0 || bandCount <= 0 {
		return &EnergyHistogram{BinDuration: binDuration, BandCount: bandCount}
	}

	binCount := int(math.Ceil(duration / binDuration))

	bins := make([]HistogramBin, binCount)
	for i := range bins {
		bins[i] = HistogramBin{
			TimeSeconds: float64(i) * binDuration,
			BandEnergy:  make([]float64, bandCount),
		}
	}

	return &EnergyHistogram{Bins: bins, BinDuration: binDuration, BandCount: bandCount}
}

// Add accumulates band energy into the bin covering timeSeconds.
func (h *EnergyHistogram) Add(timeSeconds float64, bandEnergy []float64) {
	if h == nil || h.BinDuration <= 0 || len(h.Bins) == 0 || timeSeconds < 0 {
		return
	}

	index := int(math.Floor(timeSeconds / h.BinDuration))
	if index < 0 {
		return
	}

	if index >= len(h.Bins) {
		index = len(h.Bins) - 1
	}

	bin := &h.Bins[index]
	if len(bin.BandEnergy) != h.BandCount {
		bin.BandEnergy = make([]float64, h.BandCount)
	}

	limit := min(len(bandEnergy), h.BandCount)

	for i := range limit {
		bin.BandEnergy[i] += bandEnergy[i]
	}
}

// ToLateMono converts the histogram into a late-field mono buffer.
func (h *EnergyHistogram) ToLateMono(sampleRate int) *ir.Buffer {
	if h == nil || sampleRate <= 0 || h.BinDuration <= 0 || len(h.Bins) == 0 {
		return ir.NewBuffer(sampleRate, 0)
	}

	buf := ir.NewBuffer(sampleRate, float64(len(h.Bins))*h.BinDuration)
	rng := rand.New(rand.NewSource(1))

	for binIndex, bin := range h.Bins {
		start := int(math.Round(float64(binIndex) * h.BinDuration * float64(sampleRate)))

		end := min(int(math.Round(float64(binIndex+1)*h.BinDuration*float64(sampleRate))), len(buf.Samples))

		if start < 0 {
			start = 0
		}

		if start >= end {
			continue
		}

		var totalEnergy float64
		for _, bandEnergy := range bin.BandEnergy {
			totalEnergy += bandEnergy
		}

		if totalEnergy <= 0 {
			continue
		}

		sampleCount := end - start

		scale := math.Sqrt(3 * totalEnergy / float64(sampleCount))
		for sampleIndex := start; sampleIndex < end; sampleIndex++ {
			buf.Samples[sampleIndex] += scale * (2*rng.Float64() - 1)
		}
	}

	return buf
}
