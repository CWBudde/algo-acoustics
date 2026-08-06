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

// faceIncidentSide orients a surface normal toward the side from which dir
// arrived. Diffuse reflection must sample this hemisphere so rays in a closed
// room continue into the interior regardless of mesh winding or wall normals.
func faceIncidentSide(dir, normal geometry.Vec3) geometry.Vec3 {
	if dir.Dot(normal) > 0 {
		return normal.Scale(-1)
	}

	return normal
}

// LambertDirection returns a cosine-weighted hemisphere sample around normal.
// Uses the standard Lambert sampling: theta = arccos(sqrt(r1)), phi = 2*pi*r2.
// This ensures the pdf is proportional to cos(theta), the standard diffuse reflection model.
func LambertDirection(normal geometry.Vec3, rng *rand.Rand) geometry.Vec3 {
	if rng == nil {
		//nolint:gosec // cryptographic randomness not needed for Monte Carlo sampling
		rng = rand.New(rand.NewSource(1))
	}

	// Cosine-weighted hemisphere sampling
	r1 := rng.Float64()
	r2 := rng.Float64()

	// theta = arccos(sqrt(r1)), so cos(theta) = sqrt(r1) and sin(theta) = sqrt(1 - r1)
	cosTheta := math.Sqrt(r1)
	sinTheta := math.Sqrt(1 - r1)

	// phi = 2*pi*r2
	phi := 2 * math.Pi * r2
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	// Build local coordinate frame from normal
	n := normal.Normalize()

	// Select orthogonal axis that is most perpendicular to normal
	axis := geometry.Vec3{X: 1}
	if math.Abs(n.X) > 0.9 {
		axis = geometry.Vec3{Y: 1}
	}

	tangent := axis.Cross(n).Normalize()
	bitangent := n.Cross(tangent)

	// Construct direction in local coordinate system and transform to world
	return tangent.Scale(sinTheta * cosPhi).Add(bitangent.Scale(sinTheta * sinPhi)).Add(n.Scale(cosTheta))
}

// DiffuseReflect returns a cosine-weighted hemisphere sample around normal.
//
// Deprecated: use LambertDirection instead for correct cos-weighted sampling.
func DiffuseReflect(normal geometry.Vec3, rng *rand.Rand) geometry.Vec3 {
	return LambertDirection(normal, rng)
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
		//nolint:gosec // cryptographic randomness not needed for Monte Carlo sampling
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
