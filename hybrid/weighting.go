package hybrid

import (
	"math"

	"github.com/cwbudde/algo-acoustics/ir"
)

// LinearFade returns a linear ramp with n points.
func LinearFade(start, end int, n int) []float64 {
	if n <= 0 {
		if end > start {
			n = end - start
		} else {
			return nil
		}
	}
	if n == 1 {
		return []float64{1}
	}

	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = float64(i) / float64(n-1)
	}

	return out
}

// HannFade returns a Hann window with n points.
func HannFade(n int) []float64 {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []float64{1}
	}

	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1))
	}

	return out
}

// ApplyFade applies a crossover fade to a buffer copy.
func ApplyFade(buf *ir.Buffer, startSample, endSample int, fadeIn bool) *ir.Buffer {
	if buf == nil {
		return nil
	}
	out := cloneBuffer(buf)
	if startSample < 0 {
		startSample = 0
	}
	if endSample > len(out.Samples) {
		endSample = len(out.Samples)
	}
	if startSample >= endSample {
		return out
	}

	window := HannFade(endSample - startSample)
	for i := startSample; i < endSample; i++ {
		weight := window[i-startSample]
		if !fadeIn {
			weight = 1 - weight
		}
		out.Samples[i] *= weight
	}

	return out
}
