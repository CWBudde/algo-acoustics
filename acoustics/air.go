package acoustics

import "math"

// SpeedOfSoundAt returns the speed of sound in dry air at the given temperature
// in degrees Celsius using the Cramer (1993) approximation.
//
// Valid range: −60 °C to +60 °C.
func SpeedOfSoundAt(tempCelsius float64) float64 {
	return 331.3 * math.Sqrt(1+tempCelsius/273.15)
}

// CharacteristicImpedance returns the characteristic acoustic impedance of dry
// air at the given temperature in degrees Celsius (unit: Pa·s/m = rayl).
func CharacteristicImpedance(tempCelsius float64) float64 {
	// ρ·c; density drops as temperature rises following the ideal gas law.
	// AirDensity is the reference value at 20 °C (293.15 K).
	const refTempK = 273.15 + 20
	rho := AirDensity * refTempK / (273.15 + tempCelsius)

	return rho * SpeedOfSoundAt(tempCelsius)
}
