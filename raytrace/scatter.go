package raytrace

import (
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// ScatterConfig stores wall absorption and scattering coefficients.
type ScatterConfig struct {
	AbsorptionByBand []float64
	ScatteringByBand []float64
}

// SpecularReflect returns the mirror reflection of dir about normal.
func SpecularReflect(dir, normal geometry.Vec3) geometry.Vec3 {
	n := normal.Normalize()
	return dir.Sub(n.Scale(2 * dir.Dot(n)))
}

// DiffuseReflect returns a cosine-weighted hemisphere sample around normal.
func DiffuseReflect(normal geometry.Vec3, rng *rand.Rand) geometry.Vec3 {
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	n := normal.Normalize()
	u := math.Sqrt(rng.Float64())
	theta := 2 * math.Pi * rng.Float64()
	x := u * math.Cos(theta)
	y := u * math.Sin(theta)
	z := math.Sqrt(math.Max(0, 1-u*u))

	axis := geometry.Vec3{X: 1}
	if math.Abs(n.X) > 0.9 {
		axis = geometry.Vec3{Y: 1}
	}
	tangent := axis.Cross(n).Normalize()
	bitangent := n.Cross(tangent)

	return tangent.Scale(x).Add(bitangent.Scale(y)).Add(n.Scale(z)).Normalize()
}

// SelectReflection chooses specular or diffuse reflection based on scatterCoeff.
func SelectReflection(scatterCoeff float64, dir, normal geometry.Vec3, rng *rand.Rand) geometry.Vec3 {
	if scatterCoeff <= 0 {
		return SpecularReflect(dir, normal)
	}
	if scatterCoeff >= 1 {
		return DiffuseReflect(normal, rng)
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	if rng.Float64() < scatterCoeff {
		return DiffuseReflect(normal, rng)
	}

	return SpecularReflect(dir, normal)
}

// AbsorbedFraction returns the remaining energy fraction after absorption.
func AbsorbedFraction(absorptionByBand []float64) []float64 {
	out := make([]float64, len(absorptionByBand))
	for i, absorption := range absorptionByBand {
		remaining := 1 - absorption
		if remaining < 0 {
			remaining = 0
		}
		if remaining > 1 {
			remaining = 1
		}
		out[i] = remaining
	}

	return out
}

func averageCoeff(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, value := range values {
		sum += value
	}

	return sum / float64(len(values))
}
