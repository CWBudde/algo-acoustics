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

	n := len(geoIR.Samples)
	if len(lowIR) > n {
		n = len(lowIR)
	}
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
	if err := plan.Forward(lowFreq, lowSpec); err != nil {
		return nil
	}
	if err := plan.Forward(geoFreq, geoSpec); err != nil {
		return nil
	}

	for k := range lowFreq {
		freq := float64(k) * float64(sampleRate) / float64(fftSize)
		wLow := lowPassWeight(freq, crossoverHz)
		wHigh := 1 - wLow
		lowFreq[k] *= complex(wLow, 0)
		geoFreq[k] *= complex(wHigh, 0)
	}

	combinedFreq := make([]complex128, fftSize)
	for i := range combinedFreq {
		combinedFreq[i] = lowFreq[i] + geoFreq[i]
	}

	combinedTime := make([]complex128, fftSize)
	if err := plan.Inverse(combinedTime, combinedFreq); err != nil {
		return nil
	}

	out := ir.NewBuffer(sampleRate, float64(n)/float64(sampleRate))
	limit := len(out.Samples)
	if limit > len(combinedTime) {
		limit = len(combinedTime)
	}
	for i := 0; i < limit; i++ {
		out.Samples[i] = real(combinedTime[i])
	}

	return out
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
