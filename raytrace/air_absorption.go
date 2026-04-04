package raytrace

import "math"

const (
	defaultAirTemperatureC  = 20.0
	defaultRelativeHumidity = 0.5
	defaultAirPressureKPa   = 101.325
	referenceTemperatureK   = 293.15
	referencePressureKPa    = 101.325
	triplePointTemperatureK = 273.16
)

// AlphaAirISO9613_1 returns the ISO 9613-1 atmospheric absorption coefficient in dB/m.
// Temperature is in degrees Celsius and relative humidity is a fraction in [0, 1].
func AlphaAirISO9613_1(frequencyHz, temperatureC, relativeHumidity float64) float64 {
	if frequencyHz <= 0 {
		return 0
	}

	temperatureK := temperatureC + 273.15
	if temperatureK <= 0 {
		return 0
	}

	relativeHumidity = clamp01(relativeHumidity)

	pressureKPa := defaultAirPressureKPa
	if pressureKPa <= 0 {
		pressureKPa = referencePressureKPa
	}

	satPressure := referencePressureKPa * math.Pow(10, -6.8346*math.Pow(triplePointTemperatureK/temperatureK, 1.261)+4.6151)
	h := relativeHumidity * satPressure / pressureKPa

	frO := pressureKPa / referencePressureKPa * (24.0 + 4.04e4*h*(0.02+h)/(0.391+h))
	frN := pressureKPa / referencePressureKPa * math.Pow(temperatureK/referenceTemperatureK, -0.5) *
		(9.0 + 280.0*h*math.Exp(-4.170*(math.Pow(temperatureK/referenceTemperatureK, -1.0/3.0)-1.0)))

	temperatureRatio := temperatureK / referenceTemperatureK
	classical := 1.84e-11 * (referencePressureKPa / pressureKPa) * math.Sqrt(temperatureRatio)
	oxygen := 0.01275 * math.Exp(-2239.1/temperatureK) * (frO / (frO + (frequencyHz*frequencyHz)/frO))
	nitrogen := 0.1068 * math.Exp(-3352.0/temperatureK) * (frN / (frN + (frequencyHz*frequencyHz)/frN))

	return 8.686 * frequencyHz * frequencyHz * (classical + math.Pow(temperatureRatio, -2.5)*(oxygen+nitrogen))
}

func attenuateEnergyByAir(energy []float64, bandFreqs []float64, distanceMeters, temperatureC, relativeHumidity float64) []float64 {
	if len(energy) == 0 || distanceMeters <= 0 {
		return cloneEnergy(energy)
	}

	out := cloneEnergy(energy)
	for i := range out {
		freqHz := float64(i)
		if i < len(bandFreqs) {
			freqHz = bandFreqs[i]
		}

		alpha := AlphaAirISO9613_1(freqHz, temperatureC, relativeHumidity)
		out[i] *= math.Pow(10, -alpha*distanceMeters/10)
	}

	return out
}
