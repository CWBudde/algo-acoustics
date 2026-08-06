package raytrace

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestSplitReflectionEnergyConservesBands(t *testing.T) {
	t.Parallel()

	in := []float64{1, 2, 3}
	absorption := []float64{0.25, 0.5, 0.75}
	scattering := []float64{0.2, 0.4, 0.6}

	specular, diffuse, remaining := splitReflectionEnergy(in, absorption, scattering)

	wantSpecular := []float64{0.6, 0.6, 0.3}
	wantDiffuse := []float64{0.15, 0.4, 0.45}
	wantRemaining := []float64{0.75, 1, 0.75}

	for i := range wantSpecular {
		if diff := math.Abs(specular[i] - wantSpecular[i]); diff > 1e-12 {
			t.Fatalf("specular[%d] = %g, want %g", i, specular[i], wantSpecular[i])
		}

		if diff := math.Abs(diffuse[i] - wantDiffuse[i]); diff > 1e-12 {
			t.Fatalf("diffuse[%d] = %g, want %g", i, diffuse[i], wantDiffuse[i])
		}

		if diff := math.Abs(remaining[i] - wantRemaining[i]); diff > 1e-12 {
			t.Fatalf("remaining[%d] = %g, want %g", i, remaining[i], wantRemaining[i])
		}

		if diff := math.Abs((specular[i] + diffuse[i]) - remaining[i]); diff > 1e-12 {
			t.Fatalf("band %d does not conserve energy: spec+diff=%g remaining=%g", i, specular[i]+diffuse[i], remaining[i])
		}
	}
}

func TestSampleProbabilisticReflectionPreservesPostAbsorptionEnergy(t *testing.T) {
	t.Parallel()

	const (
		samples = 20000
		scatter = 0.35
	)

	specularDir := geometry.Vec3{X: 1}
	diffuseDir := geometry.Vec3{Y: 1}
	remaining := []float64{0.9, 0.55, 0.1}
	rng := rand.New(rand.NewSource(7))
	totals := make([]float64, len(remaining))
	diffuseCount := 0

	for range samples {
		dir, energy, diffuse := sampleProbabilisticReflection(specularDir, diffuseDir, remaining, scatter, rng)
		if diffuse {
			diffuseCount++

			if dir != diffuseDir {
				t.Fatalf("diffuse sample direction = %#v, want %#v", dir, diffuseDir)
			}
		} else if dir != specularDir {
			t.Fatalf("specular sample direction = %#v, want %#v", dir, specularDir)
		}

		for bandIndex, value := range energy {
			totals[bandIndex] += value
		}
	}

	observedScatter := float64(diffuseCount) / samples
	if math.Abs(observedScatter-scatter) > 0.01 {
		t.Fatalf("diffuse selection fraction = %g, want %g ± 0.01", observedScatter, scatter)
	}

	for bandIndex, total := range totals {
		want := samples * remaining[bandIndex]
		if math.Abs(total-want) > 1e-8*want {
			t.Fatalf("propagated band %d energy = %g, want %g", bandIndex, total, want)
		}
	}
}
