// Package acoustics provides physical constants, unit helpers, air properties,
// and frequency-band specifications used throughout algo-acoustics.
package acoustics

// Physical constants at standard conditions (20 °C, 101.325 kPa).
const (
	// SpeedOfSound is the speed of sound in dry air at 20 °C in m/s.
	SpeedOfSound = 343.0

	// AirDensity is the density of dry air at 20 °C in kg/m³.
	AirDensity = 1.204

	// ReferencePressurePa is the standard acoustic reference pressure (20 µPa).
	ReferencePressurePa = 20e-6
)
