package raytrace

import (
	"math/rand"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// ReflectionStrategy controls how reflected energy is propagated after a wall hit.
type ReflectionStrategy int

const (
	// ReflectionStrategyProbabilistic chooses either the specular or diffuse
	// branch per reflection using the mean scattering coefficient.
	ReflectionStrategyProbabilistic ReflectionStrategy = iota
	// ReflectionStrategyDeterministicBlend blends the specular and diffuse
	// directions using the mean scattering coefficient.
	ReflectionStrategyDeterministicBlend
	// ReflectionStrategyRussianRoulette branches into both specular and diffuse
	// paths and uses Russian roulette to cap low-energy branch growth.
	ReflectionStrategyRussianRoulette
)

// RayState carries the geometric ray plus its per-band energy and path state.
type RayState struct {
	Ray          geometry.Ray
	Energy       []float64
	PathLength   float64
	DelaySeconds float64
	Bounces      int
	RainCovered  bool // true when this ray's energy was already deposited by diffuse rain
}

func splitReflectionEnergy(in, absorption, scattering []float64) (specular, diffuse, remaining []float64) {
	limit := len(in)
	specular = make([]float64, limit)
	diffuse = make([]float64, limit)
	remaining = make([]float64, limit)

	for i := range limit {
		absorbed := 0.0
		if i < len(absorption) {
			absorbed = clamp01(absorption[i])
		}

		scatter := 0.0
		if i < len(scattering) {
			scatter = clamp01(scattering[i])
		}

		left := in[i] * (1 - absorbed)
		remaining[i] = left
		specular[i] = left * (1 - scatter)
		diffuse[i] = left * scatter
	}

	return specular, diffuse, remaining
}

func maxEnergy(values []float64) float64 {
	var maximum float64
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}

	return maximum
}

func cloneEnergy(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}

	return append([]float64(nil), values...)
}

func chooseBlendDirection(specularDir, diffuseDir geometry.Vec3, scatter float64) geometry.Vec3 {
	scatter = clamp01(scatter)

	dir := specularDir.Scale(1 - scatter).Add(diffuseDir.Scale(scatter))
	if dir == geometry.Vec3Zero {
		return specularDir
	}

	return dir.Normalize()
}

func sampleProbabilisticReflection(
	specularDir, diffuseDir geometry.Vec3,
	remainingEnergy []float64,
	scatter float64,
	rng *rand.Rand,
) (geometry.Vec3, []float64, bool) {
	scatter = clamp01(scatter)
	if scatter >= 1 || (scatter > 0 && rng.Float64() < scatter) {
		return diffuseDir, remainingEnergy, true
	}

	return specularDir, remainingEnergy, false
}

func russianRouletteEnergy(energy []float64, threshold float64, rng *rand.Rand) ([]float64, bool) {
	if len(energy) == 0 {
		return nil, false
	}

	if threshold <= 0 {
		return cloneEnergy(energy), true
	}

	peak := maxEnergy(energy)
	if peak > threshold {
		return cloneEnergy(energy), true
	}

	if rng == nil {
		//nolint:gosec // cryptographic randomness not needed for Monte Carlo branching
		rng = rand.New(rand.NewSource(1))
	}

	survival := peak / threshold
	if survival < 0.05 {
		survival = 0.05
	}

	if rng.Float64() > survival {
		return nil, false
	}

	out := cloneEnergy(energy)

	scale := 1 / survival
	for i := range out {
		out[i] *= scale
	}

	return out, true
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
