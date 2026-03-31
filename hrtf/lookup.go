package hrtf

import "github.com/cwbudde/algo-acoustics/geometry"

// NearestNeighborDataset is a stub dataset that carries a sample rate and
// provides a placeholder lookup implementation until the full HRTF backend
// lands in Phase 7.
type NearestNeighborDataset struct {
	SampleRateHz int `json:"sampleRate"`
}

// SampleRate returns the dataset sample rate.
func (d NearestNeighborDataset) SampleRate() int {
	return d.SampleRateHz
}

// Lookup is a placeholder nearest-neighbor scaffold.
func (d NearestNeighborDataset) Lookup(direction geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	return nil, nil, 0, nil
}
