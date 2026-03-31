package scene

// Material describes band-dependent absorption and scattering properties.
type Material struct {
	Name             string    `json:"name"`
	AbsorptionByBand []float64 `json:"absorptionByBand,omitempty"`
	ScatteringByBand []float64 `json:"scatteringByBand,omitempty"`
}

// AbsorptionAt returns the absorption coefficient for the requested band.
func (m Material) AbsorptionAt(bandIndex int) float64 {
	if bandIndex < 0 || bandIndex >= len(m.AbsorptionByBand) {
		return 0
	}

	return m.AbsorptionByBand[bandIndex]
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
