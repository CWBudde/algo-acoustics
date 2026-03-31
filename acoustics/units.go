package acoustics

import "math"

// DecibelToLinear converts a decibel value to a linear amplitude ratio.
func DecibelToLinear(dB float64) float64 {
	return math.Pow(10, dB/20)
}

// LinearToDecibel converts a linear amplitude ratio to decibels.
// Returns -Inf for zero or negative values.
func LinearToDecibel(linear float64) float64 {
	if linear <= 0 {
		return math.Inf(-1)
	}

	return 20 * math.Log10(linear)
}

// MetersToSamples converts a distance in metres to a sample offset given the
// speed of sound c (m/s) and the sample rate.
func MetersToSamples(m, c float64, sampleRate int) int {
	return int(m / c * float64(sampleRate))
}

// SamplesToSeconds converts a sample count to a duration in seconds.
func SamplesToSeconds(n, sampleRate int) float64 {
	return float64(n) / float64(sampleRate)
}
