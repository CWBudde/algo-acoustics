package hrtf

import "github.com/cwbudde/algo-acoustics/geometry"

// NoopDataset returns a centered identity HRIR for testing and fallbacks.
type NoopDataset struct {
	SampleRateHz int `json:"sampleRate"`
}

// SampleRate returns the dataset sample rate.
func (d NoopDataset) SampleRate() int {
	return d.SampleRateHz
}

// Lookup returns a single-sample identity impulse in both channels.
func (d NoopDataset) Lookup(direction geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	return []float64{1}, []float64{1}, 0, nil
}