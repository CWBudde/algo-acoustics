package directivity

import "github.com/cwbudde/algo-acoustics/geometry"

// OmniModel is an omnidirectional directivity pattern.
type OmniModel struct{}

// GainLinear always returns unity gain.
func (OmniModel) GainLinear(_ float64, _ geometry.Vec3) float64 {
	return 1
}
