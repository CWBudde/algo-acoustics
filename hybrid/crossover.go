package hybrid

import (
	"math"

	"github.com/cwbudde/algo-acoustics/ir"
	algofft "github.com/cwbudde/algo-fft"
)

// BlendLowFreq combines a low-frequency signal with a geometric IR using a smooth crossover.
func BlendLowFreq(lowIR []float64, geoIR *ir.Buffer, crossoverHz float64, sampleRate int) *ir.Buffer {
	if geoIR == nil {
		return nil
	}

	if len(lowIR) == 0 {
		return cloneBuffer(geoIR)
	}

	if sampleRate <= 0 || geoIR.SampleRate != sampleRate {
		return nil
	}

	n := max(len(lowIR), len(geoIR.Samples))

	fftSize := nextPowerOf2(maxInt(2*n, 2))

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil
	}

	lowSpec, geoSpec := make([]complex128, fftSize), make([]complex128, fftSize)
	for i, v := range lowIR {
		lowSpec[i] = complex(v, 0)
	}

	for i, v := range geoIR.Samples {
		geoSpec[i] = complex(v, 0)
	}

	lowFreq, geoFreq := make([]complex128, fftSize), make([]complex128, fftSize)

	err = plan.Forward(lowFreq, lowSpec)
	if err != nil {
		return nil
	}

	err = plan.Forward(geoFreq, geoSpec)
	if err != nil {
		return nil
	}

	combinedFreq := blendFrequencySpectra(lowFreq, geoFreq, crossoverHz, sampleRate)

	combinedTime := make([]complex128, fftSize)

	err = plan.Inverse(combinedTime, combinedFreq)
	if err != nil {
		return nil
	}

	out := ir.NewBuffer(sampleRate, float64(n)/float64(sampleRate))

	limit := min(len(out.Samples), len(combinedTime))

	for i := range limit {
		out.Samples[i] = real(combinedTime[i])
	}

	return out
}

func blendFrequencySpectra(lowFreq, geoFreq []complex128, crossoverHz float64, sampleRate int) []complex128 {
	fftSize := len(lowFreq)
	combined := make([]complex128, fftSize)

	for k := range combined {
		// Fold negative-frequency bins onto their positive-frequency partners.
		freq := float64(min(k, fftSize-k)) * float64(sampleRate) / float64(fftSize)
		wLow := lowPassWeight(freq, crossoverHz)
		combined[k] = lowFreq[k]*complex(wLow, 0) + geoFreq[k]*complex(1-wLow, 0)
	}

	return combined
}

func lowPassWeight(freq, crossoverHz float64) float64 {
	if crossoverHz <= 0 {
		return 1
	}

	if freq <= crossoverHz*0.5 {
		return 1
	}

	if freq >= crossoverHz*2 {
		return 0
	}

	x := (math.Log2(freq/crossoverHz) + 1) / 2

	return 0.5 * (1 + math.Cos(math.Pi*x))
}

func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}

	power := 1
	for power < n {
		power <<= 1
	}

	return power
}
