package raytrace

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// DiffuseRainContribution holds the energy and arrival time of a single
// diffuse rain deposit into the energy histogram.
type DiffuseRainContribution struct {
	BandEnergy  []float64
	ArrivalTime float64
}

// computeDiffuseRain calculates the secondary radiation energy from a diffuse
// reflection point toward the receiver sphere. This implements the RAVEN
// "diffuse rain" variance-reduction technique (Schroeder 2011, Eq. 5.20):
//
//	E_s = E_P * s * (1 - cos(gamma/2)) * 2 * cos(Theta) * exp(-m*r)
//
// where E_P is the per-band particle energy (already after wall absorption),
// s is the per-band scattering coefficient, gamma is the receiver opening
// angle, Theta is the angle between surface normal and receiver direction,
// and r is the distance from reflection point to receiver center.
//
// If the receiver is not visible from the reflection point (occluded by
// geometry), nil is returned.
func computeDiffuseRain(
	reflectionPoint, surfaceNormal geometry.Vec3,
	energy, scattering []float64,
	receiver SphereReceiver,
	tracer Tracer,
	centerFreqs []float64,
	pathLength float64,
	speedOfSound float64,
) *DiffuseRainContribution {
	if len(energy) == 0 || receiver.Radius <= 0 || speedOfSound <= 0 {
		return nil
	}

	// Direction and distance from reflection point to receiver center.
	toReceiver := receiver.Center.Sub(reflectionPoint)
	dist := toReceiver.Norm()

	if dist <= 0 {
		return nil
	}

	dir := toReceiver.Scale(1 / dist)

	// Lambert cosine factor: cos(Theta) between surface normal and
	// the direction toward the receiver. Negative means the receiver
	// is behind the surface — no rain contribution.
	cosTheta := surfaceNormal.Dot(dir)
	if cosTheta <= 0 {
		return nil
	}

	// Visibility check: cast a ray from the reflection point toward
	// the receiver. If geometry is hit before reaching the receiver
	// sphere, the path is occluded.
	if tracer != nil {
		origin := reflectionPoint.Add(dir.Scale(wallEpsilon))
		ray := geometry.NewRay(origin, dir)

		hitPoint, _, _, hit := tracer.NextHit(ray)
		if hit {
			hitDist := origin.Distance(hitPoint)
			receiverEntry := dist - receiver.Radius

			if hitDist < receiverEntry-wallEpsilon {
				return nil
			}
		}
	}

	// Detector solid angle: opening angle gamma of the sphere as seen
	// from the reflection point.
	ratio := receiver.Radius / dist
	if ratio > 1 {
		ratio = 1
	}

	gamma := math.Asin(ratio)
	cosHalfGamma := math.Cos(gamma / 2)
	solidAngleFactor := (1 - cosHalfGamma) * 2

	// Compute per-band rain energy.
	bandEnergy := make([]float64, len(energy))

	for i := range energy {
		s := 1.0
		if i < len(scattering) {
			s = clamp01(scattering[i])
		}

		if s <= 0 || energy[i] <= 0 {
			continue
		}

		// Air absorption over the rain path.
		freqHz := float64(i)
		if i < len(centerFreqs) {
			freqHz = centerFreqs[i]
		}

		alpha := AlphaAirISO9613_1(freqHz, defaultAirTemperatureC, defaultRelativeHumidity)
		airAtten := math.Pow(10, -alpha*dist/10)

		bandEnergy[i] = energy[i] * s * solidAngleFactor * cosTheta * airAtten
	}

	arrivalTime := (pathLength + dist) / speedOfSound

	return &DiffuseRainContribution{
		BandEnergy:  bandEnergy,
		ArrivalTime: arrivalTime,
	}
}
