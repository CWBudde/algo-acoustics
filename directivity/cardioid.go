package directivity

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// CardioidModel is a simple power-cardioid directivity pattern.
//
//nolint:tagliatelle // orderN is part of the established scene JSON schema.
type CardioidModel struct {
	Axis   geometry.Vec3 `json:"axis"`
	OrderN float64       `json:"orderN"`
}

// GainLinear returns ((1 + cos(theta)) / 2)^N for the supplied direction.
func (m CardioidModel) GainLinear(_ float64, dir geometry.Vec3) float64 {
	return cardioidGain(m.Axis, dir, m.OrderN)
}

// cardioidGain evaluates the power-cardioid pattern shared by the
// frequency-independent and frequency-dependent models.
func cardioidGain(axis, dir geometry.Vec3, order float64) float64 {
	unitAxis := axis.Normalize()
	if unitAxis == geometry.Vec3Zero {
		return 0
	}

	unitDir := dir.Normalize()
	if unitDir == geometry.Vec3Zero {
		return 0
	}

	gain := (1 + unitAxis.Dot(unitDir)) / 2
	if gain < 0 {
		gain = 0
	}

	return math.Pow(gain, order)
}
