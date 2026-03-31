package acoustics

import "math"

// BandSpec describes a set of frequency bands used for band-dependent material
// properties and energy accumulation throughout the acoustics pipeline.
type BandSpec struct {
	// CenterFreqs holds the nominal centre frequency of each band in Hz.
	CenterFreqs []float64
	// LowerEdges holds the lower band-edge frequency in Hz (same length as CenterFreqs).
	LowerEdges []float64
	// UpperEdges holds the upper band-edge frequency in Hz (same length as CenterFreqs).
	UpperEdges []float64
}

// BandCount returns the number of frequency bands in the spec.
func (b BandSpec) BandCount() int {
	return len(b.CenterFreqs)
}

// Octave6 is a 6-band, 1-octave-spaced spec covering 125 Hz – 4 kHz.
// This is the minimum band set for standard room-acoustics metrics.
var Octave6 = BandSpec{
	CenterFreqs: []float64{125, 250, 500, 1000, 2000, 4000},
	LowerEdges:  octaveLower([]float64{125, 250, 500, 1000, 2000, 4000}),
	UpperEdges:  octaveUpper([]float64{125, 250, 500, 1000, 2000, 4000}),
}

// Octave8 is an 8-band, 1-octave-spaced spec covering 63 Hz – 8 kHz.
var Octave8 = BandSpec{
	CenterFreqs: []float64{63, 125, 250, 500, 1000, 2000, 4000, 8000},
	LowerEdges:  octaveLower([]float64{63, 125, 250, 500, 1000, 2000, 4000, 8000}),
	UpperEdges:  octaveUpper([]float64{63, 125, 250, 500, 1000, 2000, 4000, 8000}),
}

// octaveLower computes the lower 1-octave band edge: f / sqrt(2).
func octaveLower(centers []float64) []float64 {
	out := make([]float64, len(centers))
	for i, f := range centers {
		out[i] = f / math.Sqrt2
	}

	return out
}

// octaveUpper computes the upper 1-octave band edge: f * sqrt(2).
func octaveUpper(centers []float64) []float64 {
	out := make([]float64, len(centers))
	for i, f := range centers {
		out[i] = f * math.Sqrt2
	}

	return out
}
