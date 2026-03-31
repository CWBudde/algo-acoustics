package directivity

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// CardioidModel is a simple power-cardioid directivity pattern.
type CardioidModel struct {
	Axis   geometry.Vec3 `json:"axis"`
	OrderN float64       `json:"orderN"`
}

// GainLinear returns ((1 + cos(theta)) / 2)^N for the supplied direction.
func (m CardioidModel) GainLinear(freqHz float64, dir geometry.Vec3) float64 {
	axis := m.Axis.Normalize()
	if axis == geometry.Vec3Zero {
		return 0
	}

	unitDir := dir.Normalize()
	if unitDir == geometry.Vec3Zero {
		return 0
	}

	gain := (1 + axis.Dot(unitDir)) / 2
	if gain < 0 {
		gain = 0
	}

	return math.Pow(gain, m.OrderN)
}
