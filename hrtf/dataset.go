package hrtf

import "github.com/cwbudde/algo-acoustics/geometry"

// Dataset is a binaural HRTF lookup interface.
type Dataset interface {
	SampleRate() int
	Lookup(direction geometry.Vec3) (left, right []float64, delaySeconds float64, err error)
}
