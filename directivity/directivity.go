package directivity

import "github.com/cwbudde/algo-acoustics/geometry"

// Model is the source directivity interface used by scene.Source.
type Model interface {
	GainLinear(freqHz float64, dir geometry.Vec3) float64
}
