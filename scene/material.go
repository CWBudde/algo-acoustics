package scene

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

// NumBands defines the default octave-band count for scattering data (125 Hz-4 kHz).
const NumBands = 6

// Material describes band-dependent absorption and scattering properties.
//
//nolint:recvcheck,tagliatelle // Mutable JSON receiver; camel-case tags preserve the public scene schema.
type Material struct {
	Name             string            `json:"name"`
	AbsorptionByBand []float64         `json:"absorptionByBand,omitempty"`
	Scattering       [NumBands]float64 `json:"scattering,omitempty"`
	ScatteringByBand []float64         `json:"scatteringByBand,omitempty"`
}

//nolint:tagliatelle // This compatibility payload mirrors the public scene schema.
type materialJSON struct {
	Name             string    `json:"name"`
	AbsorptionByBand []float64 `json:"absorptionByBand,omitempty"`
	Scattering       []float64 `json:"scattering,omitempty"`
	ScatteringByBand []float64 `json:"scatteringByBand,omitempty"`
}

// MarshalJSON omits the fixed-size scattering field when all coefficients are 0.
func (m Material) MarshalJSON() ([]byte, error) {
	payload := materialJSON{
		Name:             m.Name,
		AbsorptionByBand: m.AbsorptionByBand,
	}

	if hasNonZeroScattering(m.Scattering) {
		payload.Scattering = append([]float64(nil), m.Scattering[:]...)
	}

	if hasNonZeroFloatSlice(m.ScatteringByBand) {
		payload.ScatteringByBand = append([]float64(nil), m.ScatteringByBand...)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal material %q: %w", m.Name, err)
	}

	return encoded, nil
}

// UnmarshalJSON supports both the new "scattering" field and legacy
// "scatteringByBand" scenes.
func (m *Material) UnmarshalJSON(data []byte) error {
	var payload materialJSON

	err := json.Unmarshal(data, &payload)
	if err != nil {
		return fmt.Errorf("unmarshal material: %w", err)
	}

	m.Name = payload.Name
	m.AbsorptionByBand = append([]float64(nil), payload.AbsorptionByBand...)

	m.ScatteringByBand = nil
	for i := range m.Scattering {
		m.Scattering[i] = 0
	}

	if len(payload.Scattering) > 0 {
		copy(m.Scattering[:], payload.Scattering)
		m.ScatteringByBand = append([]float64(nil), payload.Scattering...)
	}

	// The variable-length field is authoritative when both representations are
	// present. This preserves coefficients beyond the legacy six-band array.
	if len(payload.ScatteringByBand) > 0 {
		m.ScatteringByBand = append([]float64(nil), payload.ScatteringByBand...)
		copy(m.Scattering[:], payload.ScatteringByBand)
	}

	return nil
}

// AbsorptionAt returns the absorption coefficient for the requested band.
func (m Material) AbsorptionAt(bandIndex int) float64 {
	if bandIndex < 0 || len(m.AbsorptionByBand) == 0 {
		return 0
	}

	if len(m.AbsorptionByBand) == 1 {
		return m.AbsorptionByBand[0]
	}

	if bandIndex >= len(m.AbsorptionByBand) {
		return 0
	}

	return m.AbsorptionByBand[bandIndex]
}

// ScatteringCoefficients returns per-band scattering coefficients for the
// requested band count.
func (m Material) ScatteringCoefficients(bandCount int) []float64 {
	if bandCount <= 0 {
		return nil
	}

	if len(m.ScatteringByBand) == bandCount {
		return append([]float64(nil), m.ScatteringByBand...)
	}

	if len(m.ScatteringByBand) > 0 {
		out := make([]float64, bandCount)

		copied := copy(out, m.ScatteringByBand)
		if copied > 0 {
			fill := out[copied-1]
			for i := copied; i < len(out); i++ {
				out[i] = fill
			}
		}

		return out
	}

	out := make([]float64, bandCount)

	copied := copy(out, m.Scattering[:])
	if copied > 0 {
		fill := out[copied-1]
		for i := copied; i < len(out); i++ {
			out[i] = fill
		}
	}

	return out
}

// EstimateScatteringFromDepth estimates default scattering coefficients from
// structural depth using the model s(f) = 1 - exp(-k * (f / f0)^2).
func EstimateScatteringFromDepth(depthMeters float64) [NumBands]float64 {
	return EstimateScatteringFromDepthWithK(depthMeters, 1)
}

// EstimateScatteringFromDepthWithK estimates scattering using a configurable k.
func EstimateScatteringFromDepthWithK(depthMeters, k float64) [NumBands]float64 {
	var scattering [NumBands]float64
	if depthMeters <= 0 || k <= 0 {
		return scattering
	}

	f0 := acoustics.SpeedOfSound / (2 * depthMeters)
	for i, f := range acoustics.Octave6.CenterFreqs {
		ratio := f / f0

		s := 1 - math.Exp(-k*ratio*ratio)
		if s < 0 {
			s = 0
		}

		if s > 1 {
			s = 1
		}

		scattering[i] = s
	}

	return scattering
}

func hasNonZeroScattering(scattering [NumBands]float64) bool {
	for _, coeff := range scattering {
		if coeff != 0 {
			return true
		}
	}

	return false
}

func hasNonZeroFloatSlice(values []float64) bool {
	for _, value := range values {
		if value != 0 {
			return true
		}
	}

	return false
}

// MaterialFullyAbsorptive returns a convenience material that absorbs all energy.
func MaterialFullyAbsorptive() Material {
	return Material{
		Name:             "fully_absorptive",
		AbsorptionByBand: []float64{1},
		ScatteringByBand: []float64{0},
	}
}

// MaterialFullyReflective returns a convenience material that reflects all energy.
func MaterialFullyReflective() Material {
	return Material{
		Name:             "fully_reflective",
		AbsorptionByBand: []float64{0},
		ScatteringByBand: []float64{0},
	}
}
