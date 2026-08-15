package directivity

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// FrequencyDependentCardioid is a power-cardioid whose order varies with
// frequency, so the pattern can be wide at low frequencies and narrow at high
// ones — the behaviour of real instruments and loudspeakers, which the
// frequency-independent CardioidModel cannot express.
//
// Bands holds ascending centre frequencies in Hz and Orders the cardioid
// exponent at each of them. Between tabulated bands the order is interpolated
// linearly in log frequency, which keeps the transition even across octaves;
// outside them the nearest endpoint order is held.
//
// A model with no bands radiates omnidirectionally, matching CardioidModel at
// order 0.
type FrequencyDependentCardioid struct {
	Axis   geometry.Vec3 `json:"axis"`
	Bands  []float64     `json:"bands"`
	Orders []float64     `json:"orders"`
}

// NewFrequencyDependentCardioid validates the tabulated orders and returns the
// model. Bands must be ascending and positive, and orders non-negative.
func NewFrequencyDependentCardioid(
	axis geometry.Vec3,
	bands, orders []float64,
) (FrequencyDependentCardioid, error) {
	if len(bands) == 0 {
		return FrequencyDependentCardioid{}, errors.New("frequency-dependent cardioid has no bands")
	}

	if len(bands) != len(orders) {
		return FrequencyDependentCardioid{}, fmt.Errorf(
			"frequency-dependent cardioid has %d bands but %d orders", len(bands), len(orders))
	}

	for index, freq := range bands {
		if freq <= 0 {
			return FrequencyDependentCardioid{}, fmt.Errorf("band %d frequency %v is not positive", index, freq)
		}

		if index > 0 && freq <= bands[index-1] {
			return FrequencyDependentCardioid{}, fmt.Errorf(
				"band %d frequency %v does not ascend past %v", index, freq, bands[index-1])
		}

		if orders[index] < 0 {
			return FrequencyDependentCardioid{}, fmt.Errorf("band %d order %v is negative", index, orders[index])
		}
	}

	return FrequencyDependentCardioid{
		Axis:   axis,
		Bands:  append([]float64(nil), bands...),
		Orders: append([]float64(nil), orders...),
	}, nil
}

// OrderAt returns the cardioid order interpolated to the supplied frequency.
func (m FrequencyDependentCardioid) OrderAt(freqHz float64) float64 {
	if len(m.Bands) == 0 || len(m.Orders) == 0 {
		return 0
	}

	count := min(len(m.Bands), len(m.Orders))

	if freqHz <= m.Bands[0] {
		return m.Orders[0]
	}

	if freqHz >= m.Bands[count-1] {
		return m.Orders[count-1]
	}

	upper := 1
	for upper < count && m.Bands[upper] < freqHz {
		upper++
	}

	lower := upper - 1

	span := math.Log(m.Bands[upper] / m.Bands[lower])
	if span <= 0 {
		return m.Orders[lower]
	}

	weight := math.Log(freqHz/m.Bands[lower]) / span

	return m.Orders[lower] + weight*(m.Orders[upper]-m.Orders[lower])
}

// GainLinear returns ((1 + cos(theta)) / 2)^N with N taken at freqHz.
func (m FrequencyDependentCardioid) GainLinear(freqHz float64, dir geometry.Vec3) float64 {
	return cardioidGain(m.Axis, dir, m.OrderAt(freqHz))
}
